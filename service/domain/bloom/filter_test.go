package bloom

import (
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/stretchr/testify/require"
)

func TestEventFilter(t *testing.T) {
	config := FilterConfig{
		ExpectedItems:     1000,
		FalsePositiveRate: 0.01,
		RotationInterval:  time.Hour,
	}

	filter := NewEventFilter(config)

	// Test adding and checking an event
	event1 := fixtures.SomeEvent()
	filter.Add(event1.Id())
	require.True(t, filter.Contains(event1.Id()))

	// Test a non-existent event
	event2 := fixtures.SomeEvent()
	require.False(t, filter.Contains(event2.Id()))

	// Test rotation
	originalFilter := filter.current
	filter.lastRotated = time.Now().Add(-2 * time.Hour) // Force rotation
	event3 := fixtures.SomeEvent()
	filter.Add(event3.Id())

	// After rotation:
	// 1. We should have a new current filter
	require.NotEqual(t, originalFilter, filter.current)
	// 2. The old filter should be the previous one
	require.Equal(t, originalFilter, filter.previous)
	// 3. Both old and new events should be findable
	require.True(t, filter.Contains(event1.Id()))
	require.True(t, filter.Contains(event3.Id()))

	// Test stats
	stats := filter.Stats()
	require.Greater(t, stats.CurrentSize, uint(0))
	require.Greater(t, stats.PreviousSize, uint(0))
	require.False(t, stats.LastRotated.IsZero())
}

func TestEventFilter_Concurrent(t *testing.T) {
	config := FilterConfig{
		ExpectedItems:     1000,
		FalsePositiveRate: 0.01,
		RotationInterval:  time.Hour,
	}

	filter := NewEventFilter(config)
	done := make(chan struct{})

	// Start multiple goroutines to add and check events
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				event := fixtures.SomeEvent()
				filter.Add(event.Id())
				_ = filter.Contains(event.Id())
				_ = filter.Stats()
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}
