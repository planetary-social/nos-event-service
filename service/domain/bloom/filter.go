package bloom

import (
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/planetary-social/nos-event-service/service/domain"
)

// FilterConfig holds configuration for the Bloom filter
type FilterConfig struct {
	// ExpectedItems is the expected number of items to be added to the filter
	ExpectedItems uint
	// FalsePositiveRate is the desired false positive rate (between 0 and 1)
	FalsePositiveRate float64
	// RotationInterval is how often we rotate to a new filter
	RotationInterval time.Duration
}

// EventFilter uses a pair of rotating Bloom filters to detect duplicate events
type EventFilter struct {
	current     *bloom.BloomFilter
	previous    *bloom.BloomFilter
	config      FilterConfig
	lastRotated time.Time
	mu          sync.RWMutex
}

// NewEventFilter creates a new event filter with the given configuration
func NewEventFilter(config FilterConfig) *EventFilter {
	filter := &EventFilter{
		config:      config,
		lastRotated: time.Now(),
	}
	filter.current = bloom.NewWithEstimates(uint(config.ExpectedItems), config.FalsePositiveRate)
	filter.previous = bloom.NewWithEstimates(uint(config.ExpectedItems), config.FalsePositiveRate)
	return filter
}

// Add adds an event ID to the current filter
func (f *EventFilter) Add(eventID domain.EventId) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.maybeRotate()
	f.current.Add(eventID.Bytes())
}

// Contains checks if an event ID exists in either the current or previous filter
func (f *EventFilter) Contains(eventID domain.EventId) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.current.Test(eventID.Bytes()) || f.previous.Test(eventID.Bytes())
}

// maybeRotate rotates the filters if the rotation interval has elapsed
// Caller must hold the write lock
func (f *EventFilter) maybeRotate() {
	if time.Since(f.lastRotated) >= f.config.RotationInterval {
		// Previous filter is discarded, current becomes previous, and we create a new current
		f.previous = f.current
		f.current = bloom.NewWithEstimates(uint(f.config.ExpectedItems), f.config.FalsePositiveRate)
		f.lastRotated = time.Now()
	}
}

// Stats returns current statistics about the filter
func (f *EventFilter) Stats() FilterStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return FilterStats{
		CurrentSize:  uint(f.current.ApproximatedSize()),
		PreviousSize: uint(f.previous.ApproximatedSize()),
		LastRotated:  f.lastRotated,
	}
}

// FilterStats holds statistics about the filter
type FilterStats struct {
	CurrentSize  uint
	PreviousSize uint
	LastRotated  time.Time
}

func (f *EventFilter) ExpectedItems() uint {
	return f.config.ExpectedItems
}
