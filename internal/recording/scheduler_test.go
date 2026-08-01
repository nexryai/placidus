package recording

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexryai/placidus/internal/mirakurun"
	"github.com/nexryai/placidus/internal/tuner"
)

type streamingBackend struct{}

func (streamingBackend) Services(context.Context) ([]mirakurun.Service, error) { return nil, nil }
func (streamingBackend) Programs(context.Context) ([]mirakurun.Program, error) { return nil, nil }
func (streamingBackend) Tuners(context.Context) ([]mirakurun.Tuner, error)     { return nil, nil }
func (streamingBackend) OpenServiceStream(ctx context.Context, _ uint64, _ int) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		defer writer.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := writer.Write([]byte("test-packet")); err != nil {
					return
				}
			}
		}
	}()
	return reader, nil
}

func TestSchedulerCompletesDueRecording(t *testing.T) {
	directory := t.TempDir()
	store, err := NewStore(filepath.Join(directory, "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Create(Reservation{
		ProgramID: 1, ServiceID: 100, Title: "Short Program",
		StartAt: time.Now().Add(-time.Second), EndAt: time.Now().Add(120 * time.Millisecond),
	})
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewScheduler(store, streamingBackend{}, tuner.NewCoordinator(1), filepath.Join(directory, "recordings"), 0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { scheduler.Run(ctx); close(done) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := store.Get(item.ID)
		if current.State == StateCompleted {
			info, statErr := os.Stat(filepath.Join(directory, "recordings", current.FileName))
			if statErr != nil || info.Size() == 0 {
				t.Fatalf("recording output is missing or empty: %v", statErr)
			}
			cancel()
			<-done
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	current, _ := store.Get(item.ID)
	t.Fatalf("recording did not complete: %#v", current)
}
