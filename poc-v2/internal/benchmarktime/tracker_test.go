package benchmarktime

import (
	"testing"
	"time"
)

type fakeClock struct {
	values []time.Time
	index  int
}

func (clock *fakeClock) Now() time.Time {
	value := clock.values[clock.index]
	clock.index++
	return value
}

func TestTrackerMeasuresConfiguredBoundary(t *testing.T) {
	start := time.Unix(100, 0)
	clock := &fakeClock{values: []time.Time{start, start.Add(137 * time.Millisecond)}}
	tracker := Start(clock)
	if got := tracker.Elapsed(); got != 137*time.Millisecond {
		t.Fatalf("elapsed=%s", got)
	}
}
