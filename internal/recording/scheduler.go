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
	scheduler := &Scheduler{
		store: store, backend: backend, coordinator: coordinator, recordingDir: recordingDir,
		margin: margin, logger: logger, active: make(map[string]context.CancelFunc),
	}
	logger.Info("recording scheduler initialized", "recording_directory", recordingDir, "margin", margin, "reservations", len(store.List()))
	return scheduler, nil
}

// Run reconciles persisted state, starts due work, and blocks until shutdown.
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.Info("recording scheduler started")
	s.reconcile()
	s.tick(ctx, time.Now())
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("recording scheduler stopping", "active_recordings", s.activeCount(), "reason", ctx.Err())
			s.stopAll()
			s.wg.Wait()
			s.logger.Info("recording scheduler stopped")
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
		s.logger.Warn("reservation cancellation failed", "reservation_id", id, "reason", "not found")
		return os.ErrNotExist
	}
	if item.State != StateScheduled && item.State != StateRecording {
		s.logger.Warn("reservation cancellation rejected", "reservation_id", id, "state", item.State)
		return fmt.Errorf("reservation in state %s cannot be cancelled", item.State)
	}
	if _, err := s.store.Update(id, func(current *Reservation) error {
		current.State = StateCancelled
		current.Error = ""
		return nil
	}); err != nil {
		s.logger.Error("reservation cancellation could not be persisted", "reservation_id", id, "error", err)
		return err
	}
	s.mu.Lock()
	if cancel := s.active[id]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	s.logger.Info("reservation cancelled", "reservation_id", id, "service_id", item.ServiceID, "previous_state", item.State)
	return nil
}

func (s *Scheduler) reconcile() {
	now := time.Now()
	reconciled := 0
	for _, item := range s.store.List() {
		if item.State != StateRecording {
			continue
		}
		updated, err := s.store.Update(item.ID, func(current *Reservation) error {
			if current.EndAt.Add(s.margin).After(now) {
				current.State = StateScheduled
				current.Error = ""
			} else {
				current.State = StateFailed
				current.Error = "placidus stopped before the recording completed"
			}
			return nil
		})
		if err != nil {
			s.logger.Error("reservation reconciliation failed", "reservation_id", item.ID, "error", err)
			continue
		}
		reconciled++
		s.logger.Warn("interrupted reservation reconciled", "reservation_id", item.ID, "service_id", item.ServiceID, "state", updated.State, "end_at", item.EndAt)
	}
	s.logger.Info("reservation reconciliation completed", "reconciled", reconciled)
}

func (s *Scheduler) tick(parent context.Context, now time.Time) {
	for _, item := range s.store.List() {
		if item.State != StateScheduled || item.StartAt.Add(-s.margin).After(now) {
			continue
		}
		if !item.EndAt.Add(s.margin).After(now) {
			_, err := s.store.Update(item.ID, func(current *Reservation) error {
				current.State = StateFailed
				current.Error = "reservation was not started before its end time"
				return nil
			})
			if err != nil {
				s.logger.Error("missed reservation state could not be persisted", "reservation_id", item.ID, "error", err)
			} else {
				s.logger.Error("reservation missed", "reservation_id", item.ID, "service_id", item.ServiceID, "start_at", item.StartAt, "end_at", item.EndAt)
			}
			continue
		}
		s.logger.Debug("reservation became due", "reservation_id", item.ID, "service_id", item.ServiceID, "scheduled_start", item.StartAt, "scheduled_end", item.EndAt)
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
		s.logger.Error("recording could not acquire tuner", "reservation_id", item.ID, "service_id", item.ServiceID, "error", err)
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
		s.logger.Error("recording start state could not be persisted", "reservation_id", item.ID, "service_id", item.ServiceID, "error", err)
		release()
		s.removeActive(item.ID)
		cancel()
		return
	}

	s.logger.Info("recording started", "reservation_id", item.ID, "service_id", item.ServiceID, "title", item.Title, "scheduled_end", item.EndAt, "margin", s.margin, "file", fileName)
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
	started := time.Now()
	s.logger.Debug("opening recording output", "reservation_id", item.ID, "path", path)
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
	s.logger.Info("recording stream connected", "reservation_id", item.ID, "service_id", item.ServiceID)
	bytesWritten, copyErr := io.Copy(output, stream)
	stream.Close()
	if syncErr := output.Sync(); copyErr == nil {
		copyErr = syncErr
	}

	current, ok := s.store.Get(item.ID)
	if !ok || current.State == StateCancelled {
		s.logger.Info("recording copy stopped after cancellation", "reservation_id", item.ID, "service_id", item.ServiceID, "bytes", bytesWritten, "duration", time.Since(started))
		return
	}
	if time.Now().Add(100*time.Millisecond).After(item.EndAt.Add(s.margin)) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_, _ = s.store.Update(item.ID, func(reservation *Reservation) error {
			reservation.State = StateCompleted
			reservation.Error = ""
			return nil
		})
		s.logger.Info("recording completed", "reservation_id", item.ID, "service_id", item.ServiceID, "file", path, "bytes", bytesWritten, "duration", time.Since(started))
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
		s.logger.Warn("recording interrupted for shutdown", "reservation_id", item.ID, "service_id", item.ServiceID, "bytes", bytesWritten, "duration", time.Since(started))
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

func (s *Scheduler) activeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}

func (s *Scheduler) stopAll() {
	s.mu.Lock()
	s.logger.Debug("cancelling active recording workers", "count", len(s.active))
	for _, cancel := range s.active {
		cancel()
	}
	s.mu.Unlock()
}
