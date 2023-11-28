package app

import (
	"time"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type EventFilter struct {
	filterNewerThan        *time.Duration
	filterOlderThan        *time.Duration
	filterLargerThan       *int
	filterWithMoreTagsThan *int
}

func NewEventFilter(
	filterNewerThan *time.Duration,
	filterOlderThan *time.Duration,
	filterLargerThan *int,
	filterWithMoreTagsThan *int,
) *EventFilter {
	return &EventFilter{
		filterNewerThan:        filterNewerThan,
		filterOlderThan:        filterOlderThan,
		filterLargerThan:       filterLargerThan,
		filterWithMoreTagsThan: filterWithMoreTagsThan,
	}
}

func (f EventFilter) IsOk(event Event) bool {
	if f.filterOlderThan != nil {
		maxPastAllowed := time.Now().Add(-*f.filterOlderThan)
		if event.CreatedAt().Before(maxPastAllowed) {
			return false
		}
	}

	if f.filterNewerThan != nil {
		maxFutureAllowed := time.Now().Add(*f.filterNewerThan)
		if event.CreatedAt().After(maxFutureAllowed) {
			return false
		}
	}

	if f.filterLargerThan != nil && len(event.Raw()) > *f.filterLargerThan {
		return false
	}

	if f.filterWithMoreTagsThan != nil && len(event.Tags()) > *f.filterWithMoreTagsThan {
		return false
	}

	return true
}

type Event interface {
	Id() domain.EventId
	PubKey() domain.PublicKey
	CreatedAt() time.Time
	Kind() domain.EventKind
	Tags() []domain.EventTag
	Raw() []byte
}
