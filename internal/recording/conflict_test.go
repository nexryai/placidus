package recording

import (
	"testing"
	"time"
)

func TestWouldConflictCountsDistinctConcurrentServices(t *testing.T) {
	start := time.Now().Add(time.Hour)
	existing := []Reservation{{ServiceID: 100, StartAt: start, EndAt: start.Add(time.Hour), State: StateScheduled}}
	sameService := Reservation{ServiceID: 100, StartAt: start.Add(10 * time.Minute), EndAt: start.Add(20 * time.Minute)}
	if WouldConflict(existing, sameService, 1) {
		t.Fatal("same service should share tuner capacity")
	}
	otherService := sameService
	otherService.ServiceID = 200
	if !WouldConflict(existing, otherService, 1) {
		t.Fatal("overlapping different services should conflict with one tuner")
	}
	nonOverlapping := otherService
	nonOverlapping.StartAt = start.Add(time.Hour)
	nonOverlapping.EndAt = start.Add(2 * time.Hour)
	if WouldConflict(existing, nonOverlapping, 1) {
		t.Fatal("adjacent reservations should not conflict")
	}
}
