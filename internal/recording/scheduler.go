package recording

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/tuner"
)

// Scheduler starts durable reservations and supervises their output files.
type Scheduler struct {
	store        *Store
	backend      mirakurun.API
	coordinator  *tuner.Coordinator
	recordingDir string
	margin       time.Duration
	logger       *slog.Logger

	mu     sync.Mutex
	active map[string]context.CancelFunc
	wg     sync.WaitGroup
}

// NewScheduler constructs a recording scheduler.
func NewScheduler(store *Store, backend mirakurun.API, coordinator *tuner.Coordinator, recordingDir string, margin time.Duration, logger *slog.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(recordingDir, 0o755); err != nil {
		return nil, fmt.Errorf("create recording directory: %w", err)
	}
	return &Scheduler{
		store: store, backend: backend, coordinator: coordinator, recordingDir: recordingDir,
		margin: margin, logger: logger, active: make(map[string]context.CancelFunc),
	}, nil
}

// Run reconciles persisted state, starts due work, and blocks until shutdown.
func (s *Scheduler) Run(ctx context.Context) {
	s.reconcile()
	s.tick(ctx, time.Now())
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			s.wg.Wait()
			return
		case now := <-ticker.C:
			s.tick(ctx, now)
		}
	}
}

// Cancel stops an active recording or cancels a future reservation.
func (s *Scheduler) Cancel(id string) error {
	item, ok := s.store.Get(id)
	if !ok {
		return os.ErrNotExist
	}
	if item.State != StateScheduled && item.State != StateRecording {
		return fmt.Errorf("reservation in state %s cannot be cancelled", item.State)
	}
	if _, err := s.store.Update(id, func(current *Reservation) error {
		current.State = StateCancelled
		current.Error = ""
		return nil
	}); err != nil {
		return err
	}
	s.mu.Lock()
	if cancel := s.active[id]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	return nil
}

func (s *Scheduler) reconcile() {
	now := time.Now()
	for _, item := range s.store.List() {
		if item.State != StateRecording {
			continue
		}
		_, _ = s.store.Update(item.ID, func(current *Reservation) error {
			if current.EndAt.Add(s.margin).After(now) {
				current.State = StateScheduled
				current.Error = ""
			} else {
				current.State = StateFailed
				current.Error = "placidus stopped before the recording completed"
			}
			return nil
		})
	}
}

func (s *Scheduler) tick(parent context.Context, now time.Time) {
	for _, item := range s.store.List() {
		if item.State != StateScheduled || item.StartAt.Add(-s.margin).After(now) {
			continue
		}
		if !item.EndAt.Add(s.margin).After(now) {
			_, _ = s.store.Update(item.ID, func(current *Reservation) error {
				current.State = StateFailed
				current.Error = "reservation was not started before its end time"
				return nil
			})
			continue
		}
		s.start(parent, item)
	}
}

func (s *Scheduler) start(parent context.Context, item Reservation) {
	s.mu.Lock()
	if s.active[item.ID] != nil {
		s.mu.Unlock()
		return
	}
	recordingCtx, cancel := context.WithDeadline(parent, item.EndAt.Add(s.margin))
	s.active[item.ID] = cancel
	s.mu.Unlock()

	release, err := s.coordinator.AcquireRecording(item.ServiceID)
	if err != nil {
		s.finishStartFailure(item.ID, err)
		return
	}
	fileName := item.ID + ".ts"
	if _, err := s.store.Update(item.ID, func(current *Reservation) error {
		if current.State != StateScheduled {
			return fmt.Errorf("reservation is no longer scheduled")
		}
		current.State = StateRecording
		current.FileName = fileName
		current.Error = ""
		return nil
	}); err != nil {
		release()
		s.removeActive(item.ID)
		cancel()
		return
	}

	s.logger.Info("recording started", "reservation_id", item.ID, "service_id", item.ServiceID, "title", item.Title)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer release()
		defer cancel()
		defer s.removeActive(item.ID)
		s.record(recordingCtx, item, fileName)
	}()
}

func (s *Scheduler) record(ctx context.Context, item Reservation, fileName string) {
	path := filepath.Join(s.recordingDir, fileName)
	output, err := os.Create(path)
	if err != nil {
		s.fail(item.ID, fmt.Errorf("create recording file: %w", err))
		return
	}
	defer output.Close()

	stream, err := s.backend.OpenServiceStream(ctx, item.ServiceID, 10)
	if err != nil {
		s.fail(item.ID, err)
		return
	}
	_, copyErr := io.Copy(output, stream)
	stream.Close()
	if syncErr := output.Sync(); copyErr == nil {
		copyErr = syncErr
	}

	current, ok := s.store.Get(item.ID)
	if !ok || current.State == StateCancelled {
		return
	}
	if time.Now().Add(100*time.Millisecond).After(item.EndAt.Add(s.margin)) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_, _ = s.store.Update(item.ID, func(reservation *Reservation) error {
			reservation.State = StateCompleted
			reservation.Error = ""
			return nil
		})
		s.logger.Info("recording completed", "reservation_id", item.ID, "file", path)
		return
	}
	if ctx.Err() != nil {
		// A process shutdown before airtime ends is recoverable: leave the
		// reservation scheduled so the next process can resume it.
		_, _ = s.store.Update(item.ID, func(reservation *Reservation) error {
			reservation.State = StateScheduled
			reservation.Error = ""
			return nil
		})
		return
	}
	if copyErr == nil {
		copyErr = io.ErrUnexpectedEOF
	}
	s.fail(item.ID, fmt.Errorf("recording stream ended early: %w", copyErr))
}

func (s *Scheduler) finishStartFailure(id string, err error) {
	s.fail(id, err)
	s.mu.Lock()
	if cancel := s.active[id]; cancel != nil {
		cancel()
	}
	delete(s.active, id)
	s.mu.Unlock()
}

func (s *Scheduler) fail(id string, err error) {
	_, _ = s.store.Update(id, func(item *Reservation) error {
		if item.State == StateCancelled {
			return nil
		}
		item.State = StateFailed
		item.Error = err.Error()
		return nil
	})
	s.logger.Error("recording failed", "reservation_id", id, "error", err)
}

func (s *Scheduler) removeActive(id string) {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
}

func (s *Scheduler) stopAll() {
	s.mu.Lock()
	for _, cancel := range s.active {
		cancel()
	}
	s.mu.Unlock()
}
