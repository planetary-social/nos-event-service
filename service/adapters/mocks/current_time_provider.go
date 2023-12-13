package mocks

import (
	"time"
)

type CurrentTimeProvider struct {
	t time.Time
}

func NewCurrentTimeProvider() *CurrentTimeProvider {
	return &CurrentTimeProvider{}
}

func (c *CurrentTimeProvider) SetCurrentTime(t time.Time) {
	c.t = t
}

func (c *CurrentTimeProvider) GetCurrentTime() time.Time {
	return c.t
}
