package sync

import (
	"time"
)

// Window is one contiguous comparison slice of the checked range.
type Window struct {
	Begin time.Time
	End   time.Time
	Index int
}

// Partition splits a range into contiguous windows; the final window covers
// the remainder so no slice of the range is skipped.
func Partition(begin, until time.Time, step time.Duration) []Window {
	windows := make([]Window, 0)
	index := 0
	for start := begin; start.Before(until); start = start.Add(step) {
		end := start.Add(step)
		if end.After(until) {
			end = until
		}
		windows = append(windows, Window{Begin: start, End: end, Index: index})
		index++
	}
	return windows
}
