package downloader

import "time"

type TimeWindow struct {
	start    time.Time
	duration time.Duration
}

func NewTimeWindow(start time.Time, duration time.Duration) (TimeWindow, error) {
	return TimeWindow{start: start, duration: duration}, nil
}

func (t TimeWindow) Start() time.Time {
	return t.start
}

func (t TimeWindow) End() time.Time {
	return t.start.Add(t.duration)
}

func (t TimeWindow) Advance() TimeWindow {
	newStart := t.start.Add(t.duration)
	newWindow, err := NewTimeWindow(newStart, t.duration)
	if err != nil {
		panic(err) // guaranteed by invariants
	}
	return newWindow
}
