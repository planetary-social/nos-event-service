package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type MockExternalEventPublisher struct {
}

func NewMockExternalEventPublisher() *MockExternalEventPublisher {
	return &MockExternalEventPublisher{}
}

func (m MockExternalEventPublisher) PublishNewEventReceived(ctx context.Context, event domain.Event) error {
	return nil
}
