package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type EventRepository struct {
	ListCalls       []EventRepositoryListCall
	ListReturnValue []domain.Event
}

func NewEventRepository() *EventRepository {
	return &EventRepository{}
}

func (e EventRepository) Save(ctx context.Context, event domain.Event) error {
	panic("implement me")
}

func (e EventRepository) Get(ctx context.Context, eventID domain.EventId) (domain.Event, error) {
	panic("implement me")
}

func (e EventRepository) Exists(ctx context.Context, eventID domain.EventId) (bool, error) {
	panic("implement me")
}

func (e EventRepository) Count(ctx context.Context) (int, error) {
	panic("implement me")
}

func (e *EventRepository) List(ctx context.Context, after *domain.EventId, limit int) ([]domain.Event, error) {
	e.ListCalls = append(e.ListCalls, EventRepositoryListCall{
		After: after,
		Limit: limit,
	})
	return e.ListReturnValue, nil
}

type EventRepositoryListCall struct {
	After *domain.EventId
	Limit int
}
