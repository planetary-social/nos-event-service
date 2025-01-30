package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type EventRepository struct {
	SaveCalls       []EventRepositorySaveCall
	DeleteCalls     []EventRepositoryDeleteCall
	ListCalls       []EventRepositoryListCall
	ListReturnValue []domain.Event
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

func NewEventRepository() *EventRepository {
	return &EventRepository{}
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
