// Package tuner coordinates live and recording consumers before they reach Mirakurun.
package tuner

import (
	"fmt"
	"log/slog"
	"sync"
)

// Coordinator enforces recording priority across all application media consumers.
type Coordinator struct {
	mu         sync.Mutex
	limit      int
	recordings map[uint64]int
	logger     *slog.Logger
}

// Snapshot is an immutable view of tuner allocation state.
type Snapshot struct {
	Limit             int
	Occupied          int
	RecordingServices []uint64
}

// NewCoordinator creates a coordinator with the configured tuner capacity.
func NewCoordinator(limit int, loggers ...*slog.Logger) *Coordinator {
	if limit < 1 {
		limit = 1
	}
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	logger.Info("tuner coordinator initialized", "limit", limit)
	return &Coordinator{limit: limit, recordings: make(map[uint64]int), logger: logger}
}

// AcquireRecording reserves capacity for a recording and returns an idempotent release function.
func (c *Coordinator) AcquireRecording(serviceID uint64) (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.recordings[serviceID] == 0 && len(c.recordings) >= c.limit {
		c.logger.Warn("recording tuner allocation denied", "service_id", serviceID, "occupied", len(c.recordings), "limit", c.limit)
		return nil, fmt.Errorf("all %d configured tuners are occupied by recordings", c.limit)
	}
	c.recordings[serviceID]++
	c.logger.Info("recording tuner allocated", "service_id", serviceID, "service_consumers", c.recordings[serviceID], "occupied", len(c.recordings), "limit", c.limit)
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.recordings[serviceID]--
			if c.recordings[serviceID] <= 0 {
				delete(c.recordings, serviceID)
			}
			c.logger.Info("recording tuner released", "service_id", serviceID, "occupied", len(c.recordings), "limit", c.limit)
		})
	}, nil
}

// CanView reports whether a live service may start under the recording-first policy.
func (c *Coordinator) CanView(serviceID uint64) (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.recordings) < c.limit {
		c.logger.Debug("live tuner policy allowed service", "service_id", serviceID, "occupied", len(c.recordings), "limit", c.limit)
		return true, ""
	}
	if c.recordings[serviceID] > 0 {
		c.logger.Debug("live tuner policy allowed shared recording service", "service_id", serviceID, "occupied", len(c.recordings), "limit", c.limit)
		return true, "Tuners are occupied by recording. Only the service being recorded can be watched right now."
	}
	c.logger.Warn("live tuner policy denied service", "service_id", serviceID, "occupied", len(c.recordings), "limit", c.limit)
	return false, "Tuners are occupied by recording. Switch to a service currently being recorded or wait for recording to finish."
}

// Snapshot returns current allocation state for diagnostics and UI warnings.
func (c *Coordinator) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := Snapshot{Limit: c.limit, Occupied: len(c.recordings)}
	for serviceID := range c.recordings {
		result.RecordingServices = append(result.RecordingServices, serviceID)
	}
	return result
}
