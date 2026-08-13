package recording

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorePersistsReservationTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reservations.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(Reservation{
		ProgramID: 1,
		ServiceID: 100,
		Title:     "Evening News",
		StartAt:   time.Now().Add(time.Hour),
		EndAt:     time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.State != StateScheduled || created.ID == "" {
		t.Fatalf("created reservation = %#v", created)
	}
	_, err = store.Update(created.ID, func(item *Reservation) error {
		item.State = StateRecording
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	item, ok := reopened.Get(created.ID)
	if !ok || item.State != StateRecording {
		t.Fatalf("reopened reservation = %#v, %v", item, ok)
	}
}

func TestStoreRejectsInvalidReservation(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(Reservation{Title: "Missing service and times"}); err == nil {
		t.Fatal("Create() accepted an invalid reservation")
	}
}

func TestStoreLogsReservationStateTransition(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	store, err := NewStore(filepath.Join(t.TempDir(), "reservations.json"), logger)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Create(Reservation{
		ProgramID: 1, ServiceID: 100, Title: "Logged program",
		StartAt: time.Now().Add(time.Hour), EndAt: time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(item.ID, func(current *Reservation) error {
		current.State = StateRecording
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"reservation created", "reservation state changed", "previous_state=scheduled", "state=recording"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("logs do not contain %q: %s", expected, output.String())
		}
	}
}
