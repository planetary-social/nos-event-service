package app

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type TransactionProvider interface {
	Transact(context.Context, func(context.Context, Adapters) error) error
}

type Adapters struct {
	Events    EventRepository
	Publisher Publisher
}

type EventRepository interface {
	Save(ctx context.Context, event domain.Event) error
	Get(ctx context.Context, eventID domain.EventId) (domain.Event, error)
}

type Publisher interface {
	PublishEventSaved(ctx context.Context, id domain.EventId) error
}

type ExternalEventPublisher interface {
	PublishNewEventReceived(ctx context.Context, event domain.Event) error
}

type Application struct {
	SaveReceivedEvent *SaveReceivedEventHandler
	ProcessSavedEvent *ProcessSavedEventHandler
}

type ReceivedEvent struct {
	relay domain.RelayAddress
	event domain.Event
}

func NewReceivedEvent(relay domain.RelayAddress, event domain.Event) ReceivedEvent {
	return ReceivedEvent{relay: relay, event: event}
}

func (r ReceivedEvent) Relay() domain.RelayAddress {
	return r.relay
}

func (r ReceivedEvent) Event() domain.Event {
	return r.event
}

type ReceivedEventSubscriber interface {
	Subscribe(ctx context.Context) <-chan ReceivedEvent
}

type Metrics interface {
	StartApplicationCall(handlerName string) ApplicationCall
}

type ApplicationCall interface {
	// End accepts a pointer so that you can defer this call without wrapping it
	// in an anonymous function.
	End(err *error)
}

type RelaysExtractor interface {
	Extract(event domain.Event) ([]domain.MaybeRelayAddress, error)
}
