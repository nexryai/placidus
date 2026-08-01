package tuner

import "testing"

func TestRecordingPriorityAndSharedViewing(t *testing.T) {
	coordinator := NewCoordinator(1)
	release, err := coordinator.AcquireRecording(100)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	if allowed, warning := coordinator.CanView(100); !allowed || warning == "" {
		t.Fatalf("recorded service decision = %v, %q", allowed, warning)
	}
	if allowed, warning := coordinator.CanView(200); allowed || warning == "" {
		t.Fatalf("unrelated service decision = %v, %q", allowed, warning)
	}
	if _, err := coordinator.AcquireRecording(200); err == nil {
		t.Fatal("second service recording acquired an occupied tuner")
	}
}

func TestMultipleConsumersShareRecordedService(t *testing.T) {
	coordinator := NewCoordinator(1)
	releaseA, err := coordinator.AcquireRecording(100)
	if err != nil {
		t.Fatal(err)
	}
	releaseB, err := coordinator.AcquireRecording(100)
	if err != nil {
		t.Fatal(err)
	}
	releaseA()
	if snapshot := coordinator.Snapshot(); snapshot.Occupied != 1 {
		t.Fatalf("Occupied after first release = %d", snapshot.Occupied)
	}
	releaseB()
	if snapshot := coordinator.Snapshot(); snapshot.Occupied != 0 {
		t.Fatalf("Occupied after final release = %d", snapshot.Occupied)
	}
}
