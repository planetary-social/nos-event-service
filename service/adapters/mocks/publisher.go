package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type Publisher struct {
}

func NewPublisher() *Publisher {
	return &Publisher{}
}

func (p Publisher) PublishEventSaved(ctx context.Context, id domain.EventId) error {
	panic("implement me")
}
