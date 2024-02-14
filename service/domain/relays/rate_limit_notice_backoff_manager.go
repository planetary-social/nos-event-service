package relays

import (
	"math"
	"sync/atomic"
	"time"
)

type RateLimitNoticeBackoffManager struct {
	rateLimitNoticeCount int32
	lastBumpTime         atomic.Value
}

func NewRateLimitNoticeBackoffManager() *RateLimitNoticeBackoffManager {
	r := &RateLimitNoticeBackoffManager{
		rateLimitNoticeCount: 0,
	}

	r.updateLastBumpTime()
	return r
}

func (r *RateLimitNoticeBackoffManager) Bump() {
	timeSinceLastBump := time.Since(r.getLastBumpTime())
	if timeSinceLastBump < 500*time.Millisecond {
		// Give some time for the rate limit to be lifted before increasing the counter
		return
	}

	atomic.AddInt32(&r.rateLimitNoticeCount, 1)
	r.updateLastBumpTime()
}

func (r *RateLimitNoticeBackoffManager) IsSet() bool {
	rateLimitNoticeCount := atomic.LoadInt32(&r.rateLimitNoticeCount)
	return rateLimitNoticeCount > 0
}

const maxBackoffMs = 10000
const secondsToDecreaseRateLimitNoticeCount = 60 * 5 // 5 minutes = 300 seconds

func (r *RateLimitNoticeBackoffManager) Wait() {
	if !r.IsSet() {
		return
	}

	backoffMs := int(math.Min(float64(maxBackoffMs), math.Pow(2, float64(r.rateLimitNoticeCount))*50))

	timeSinceLastBump := time.Since(r.getLastBumpTime())
	if timeSinceLastBump > secondsToDecreaseRateLimitNoticeCount*time.Second {
		atomic.AddInt32(&r.rateLimitNoticeCount, -1)
		r.updateLastBumpTime()
	}

	if backoffMs > 0 {
		time.Sleep(time.Duration(backoffMs) * time.Millisecond)
	}
}

func (r *RateLimitNoticeBackoffManager) updateLastBumpTime() time.Time {
	t := time.Now()
	r.lastBumpTime.Store(t)
	return t
}

func (r *RateLimitNoticeBackoffManager) getLastBumpTime() time.Time {
	val := r.lastBumpTime.Load()
	if t, ok := val.(time.Time); ok {
		return t
	}

	return r.updateLastBumpTime()
}
