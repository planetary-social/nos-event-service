package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var (
	ErrNoContactsEvent = errors.New("no contacts event")
)

type TransactionProvider interface {
	Transact(context.Context, func(context.Context, Adapters) error) error
}

type Adapters struct {
	Events              EventRepository
	Relays              RelayRepository
	Contacts            ContactRepository
	PublicKeysToMonitor PublicKeysToMonitorRepository
	Publisher           Publisher
}

type EventRepository interface {
	Save(ctx context.Context, event domain.Event) error
	Get(ctx context.Context, eventID domain.EventId) (domain.Event, error)
}

type RelayRepository interface {
	Save(ctx context.Context, eventID domain.EventId, relayAddress domain.MaybeRelayAddress) error
	List(ctx context.Context) ([]domain.MaybeRelayAddress, error)
}

type ContactRepository interface {
	// GetCurrentContactsEvent returns ErrNoContactsEvent if contacts haven't
	// been for this author yet.
	GetCurrentContactsEvent(ctx context.Context, author domain.PublicKey) (domain.Event, error)

	SetContacts(ctx context.Context, event domain.Event, contacts []domain.PublicKey) error

	GetFollowees(ctx context.Context, publicKey domain.PublicKey) ([]domain.PublicKey, error)
}

type PublicKeysToMonitorRepository interface {
	Save(ctx context.Context, publicKeyToMonitor domain.PublicKeyToMonitor) error
	List(ctx context.Context) ([]domain.PublicKeyToMonitor, error)
}

type Publisher interface {
	PublishEventSaved(ctx context.Context, id domain.EventId) error
}

type ExternalEventPublisher interface {
	PublishNewEventReceived(ctx context.Context, event domain.Event) error
}

type Application struct {
	SaveReceivedEvent     *SaveReceivedEventHandler
	ProcessSavedEvent     *ProcessSavedEventHandler
	UpdateMetrics         *UpdateMetricsHandler
	AddPublicKeyToMonitor *AddPublicKeyToMonitorHandler
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
	ReportNumberOfRelayDownloaders(n int)
	ReportReceivedEvent(address domain.RelayAddress)
	ReportQueueLength(topic string, n int)
}

type ApplicationCall interface {
	// End accepts a pointer so that you can defer this call without wrapping it
	// in an anonymous function.
	End(err *error)
}

type RelaysExtractor interface {
	Extract(event domain.Event) ([]domain.MaybeRelayAddress, error)
}

type ContactsExtractor interface {
	// Extract returns domain.ErrNotContactsEvent if the event isn't an event
	// that normally contains contacts. This is to distinguish between events
	// that contain zero contacts and events that don't ever contain contacts.
	Extract(event domain.Event) ([]domain.PublicKey, error)
}

type Subscriber interface {
	EventSavedQueueLength(ctx context.Context) (int, error)
}
