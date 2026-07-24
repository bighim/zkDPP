package benchmarktime

import "time"

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type Tracker struct {
	clock Clock
	start time.Time
}

func Start(clock Clock) Tracker {
	return Tracker{clock: clock, start: clock.Now()}
}

func (tracker Tracker) Elapsed() time.Duration {
	return tracker.clock.Now().Sub(tracker.start)
}
