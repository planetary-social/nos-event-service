package downloader

import (
	"time"

	"github.com/boreq/errors"
)

type TimeWindow struct {
	start    time.Time
	duration time.Duration
}

func NewTimeWindow(start time.Time, duration time.Duration) (TimeWindow, error) {
	if duration == 0 {
		return TimeWindow{}, errors.New("time window must have a duration")
	}
	return TimeWindow{start: start, duration: duration}, nil
}

func MustNewTimeWindow(start time.Time, duration time.Duration) TimeWindow {
	v, err := NewTimeWindow(start, duration)
	if err != nil {
		panic(err)
	}
	return v
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
