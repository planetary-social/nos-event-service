package bloom

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/service/domain"
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
	event1ID := makeTestEventID(t, "0000000000000000000000000000000000000000000000000000000000000001")
	filter.Add(event1ID)
	require.True(t, filter.Contains(event1ID))

	// Test a non-existent event
	event2ID := makeTestEventID(t, "0000000000000000000000000000000000000000000000000000000000000002")
	require.False(t, filter.Contains(event2ID))

	// Test rotation
	originalFilter := filter.current
	filter.lastRotated = time.Now().Add(-2 * time.Hour) // Force rotation
	event3ID := makeTestEventID(t, "0000000000000000000000000000000000000000000000000000000000000003")
	filter.Add(event3ID)

	// After rotation:
	// 1. We should have a new current filter
	require.NotEqual(t, originalFilter, filter.current)
	// 2. The old filter should be the previous one
	require.Equal(t, originalFilter, filter.previous)
	// 3. Both old and new events should be findable
	require.True(t, filter.Contains(event1ID))
	require.True(t, filter.Contains(event3ID))

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
		go func(i int) {
			for j := 0; j < 100; j++ {
				// Create deterministic but unique event IDs
				id := makeTestEventID(t, hex.EncodeToString([]byte{byte(i), byte(j), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}))
				filter.Add(id)
				_ = filter.Contains(id)
				_ = filter.Stats()
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}

func makeTestEventID(t *testing.T, hexStr string) domain.EventId {
	id, err := domain.NewEventIdFromHex(hexStr)
	require.NoError(t, err)
	return id
}
