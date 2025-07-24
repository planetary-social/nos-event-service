package mocks

import (
	"context"
	"time"

	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type EventRepository struct {
	SaveCalls                        []EventRepositorySaveCall
	DeleteCalls                      []EventRepositoryDeleteCall
	ListCalls                        []EventRepositoryListCall
	ListReturnValue                  []domain.Event
	MarkAsProcessedCalls             []EventRepositoryMarkAsProcessedCall
	DeleteProcessedEventsBeforeCalls []EventRepositoryDeleteProcessedEventsBeforeCall
	ProcessedEvents                  map[string]time.Time
}

type EventRepositorySaveCall struct {
	Event domain.Event
}

type EventRepositoryDeleteCall struct {
	EventID domain.EventId
}

type EventRepositoryListCall struct {
	After *domain.EventId
	Limit int
}

type EventRepositoryMarkAsProcessedCall struct {
	EventID domain.EventId
}

type EventRepositoryDeleteProcessedEventsBeforeCall struct {
	Before time.Time
}

func NewEventRepository() *EventRepository {
	return &EventRepository{
		ProcessedEvents: make(map[string]time.Time),
	}
}

func (e *EventRepository) Save(ctx context.Context, event domain.Event) error {
	e.SaveCalls = append(e.SaveCalls, EventRepositorySaveCall{
		Event: event,
	})
	return nil
}

func (e *EventRepository) Get(ctx context.Context, eventID domain.EventId) (domain.Event, error) {
	for _, call := range e.SaveCalls {
		if call.Event.Id() == eventID {
			return call.Event, nil
		}
	}
	return domain.Event{}, app.ErrEventNotFound
}

func (e *EventRepository) Exists(ctx context.Context, eventID domain.EventId) (bool, error) {
	for _, call := range e.SaveCalls {
		if call.Event.Id() == eventID {
			return true, nil
		}
	}
	return false, nil
}

func (e *EventRepository) Count(ctx context.Context) (int, error) {
	return len(e.SaveCalls), nil
}

func (e *EventRepository) List(ctx context.Context, after *domain.EventId, limit int) ([]domain.Event, error) {
	e.ListCalls = append(e.ListCalls, EventRepositoryListCall{
		After: after,
		Limit: limit,
	})
	return e.ListReturnValue, nil
}

func (e *EventRepository) Delete(ctx context.Context, eventID domain.EventId) error {
	e.DeleteCalls = append(e.DeleteCalls, EventRepositoryDeleteCall{
		EventID: eventID,
	})

	// Remove the event from SaveCalls
	var newSaveCalls []EventRepositorySaveCall
	for _, call := range e.SaveCalls {
		if call.Event.Id() != eventID {
			newSaveCalls = append(newSaveCalls, call)
		}
	}
	e.SaveCalls = newSaveCalls

	return nil
}

func (e *EventRepository) MarkAsProcessed(ctx context.Context, eventID domain.EventId) error {
	e.MarkAsProcessedCalls = append(e.MarkAsProcessedCalls, EventRepositoryMarkAsProcessedCall{
		EventID: eventID,
	})
	e.ProcessedEvents[eventID.Hex()] = time.Now()
	return nil
}

func (e *EventRepository) DeleteProcessedEventsBefore(ctx context.Context, before time.Time) (int, error) {
	e.DeleteProcessedEventsBeforeCalls = append(e.DeleteProcessedEventsBeforeCalls, EventRepositoryDeleteProcessedEventsBeforeCall{
		Before: before,
	})

	count := 0
	for id, processedAt := range e.ProcessedEvents {
		if processedAt.Before(before) {
			delete(e.ProcessedEvents, id)
			count++

			// Also remove from SaveCalls
			eventID, _ := domain.NewEventIdFromHex(id)
			var newSaveCalls []EventRepositorySaveCall
			for _, call := range e.SaveCalls {
				if call.Event.Id() != eventID {
					newSaveCalls = append(newSaveCalls, call)
				}
			}
			e.SaveCalls = newSaveCalls
		}
	}

	return count, nil
}
