package recording

import "time"

// WouldConflict reports whether adding candidate would exceed tunerLimit at any point.
// Multiple reservations for the same service count as one tuner because Mirakurun can
// share a tuned stream between consumers.
func WouldConflict(existing []Reservation, candidate Reservation, tunerLimit int) bool {
	if tunerLimit < 1 {
		return true
	}
	times := []time.Time{candidate.StartAt}
	for _, item := range existing {
		if item.State != StateScheduled && item.State != StateRecording {
			continue
		}
		if overlaps(item.StartAt, item.EndAt, candidate.StartAt, candidate.EndAt) {
			times = append(times, item.StartAt, item.EndAt)
		}
	}
	for _, point := range times {
		// End times are exclusive. Evaluating both the event and one nanosecond
		// after it handles starts and ends that share a timestamp.
		for _, instant := range []time.Time{point, point.Add(time.Nanosecond)} {
			if instant.Before(candidate.StartAt) || !instant.Before(candidate.EndAt) {
				continue
			}
			services := map[uint64]struct{}{candidate.ServiceID: {}}
			for _, item := range existing {
				if (item.State == StateScheduled || item.State == StateRecording) &&
					!instant.Before(item.StartAt) && instant.Before(item.EndAt) {
					services[item.ServiceID] = struct{}{}
				}
			}
			if len(services) > tunerLimit {
				return true
			}
		}
	}
	return false
}

func overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}
