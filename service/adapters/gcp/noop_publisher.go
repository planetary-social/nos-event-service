package gcp

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type NoopPublisher struct {
}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (p *NoopPublisher) PublishNewEventReceived(ctx context.Context, event domain.Event) error {
	return nil
}
