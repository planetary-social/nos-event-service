package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type RelaysExtractor struct{}

func NewRelaysExtractor() *RelaysExtractor {
	return &RelaysExtractor{}
}

func (r *RelaysExtractor) Extract(event domain.Event) ([]domain.MaybeRelayAddress, error) {
	return nil, nil
}

type ContactsExtractor struct{}

func NewContactsExtractor() *ContactsExtractor {
	return &ContactsExtractor{}
}

func (c *ContactsExtractor) Extract(event domain.Event) ([]domain.PublicKey, error) {
	return nil, nil
}

type ExternalEventPublisher struct{}

func NewExternalEventPublisher() *ExternalEventPublisher {
	return &ExternalEventPublisher{}
}

func (e *ExternalEventPublisher) PublishNewEventReceived(ctx context.Context, event domain.Event) error {
	return nil
}

type EventSender struct{}

func NewEventSender() *EventSender {
	return &EventSender{}
}

func (e *EventSender) SendEvent(ctx context.Context, address domain.RelayAddress, event domain.Event) error {
	return nil
}

func (e *EventSender) NotifyBackPressure() {}

func (e *EventSender) ResolveBackPressure() {}
