package app

import (
	"context"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var (
	ErrNoContactsEvent            = errors.New("no contacts event")
	ErrEventNotFound              = errors.New("event not found")
	ErrPublicKeyToMonitorNotFound = errors.New("public key to monitor not found")
)

const (
	applicationHandlerTimeout = 30 * time.Second
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

	// Get returns ErrEventNotFound.
	Get(ctx context.Context, eventID domain.EventId) (domain.Event, error)

	Exists(ctx context.Context, eventID domain.EventId) (bool, error)
	Count(ctx context.Context) (int, error)

	List(ctx context.Context, after *domain.EventId, limit int) ([]domain.Event, error)
}

type RelayRepository interface {
	Save(ctx context.Context, eventID domain.EventId, relayAddress domain.MaybeRelayAddress) error
	List(ctx context.Context) ([]domain.MaybeRelayAddress, error)
	Count(ctx context.Context) (int, error)
}

type ContactRepository interface {
	// GetCurrentContactsEvent returns ErrNoContactsEvent if contacts haven't
	// been for this author yet.
	GetCurrentContactsEvent(ctx context.Context, author domain.PublicKey) (domain.Event, error)

	SetContacts(ctx context.Context, event domain.Event, contacts []domain.PublicKey) error
	GetFollowees(ctx context.Context, publicKey domain.PublicKey) ([]domain.PublicKey, error)
	IsFolloweeOfMonitoredPublicKey(ctx context.Context, publicKey domain.PublicKey) (bool, error)
	CountFollowers(ctx context.Context, publicKey domain.PublicKey) (int, error)
	CountFollowees(ctx context.Context, publicKey domain.PublicKey) (int, error)
}

type PublicKeysToMonitorRepository interface {
	Save(ctx context.Context, publicKeyToMonitor domain.PublicKeyToMonitor) error
	List(ctx context.Context) ([]domain.PublicKeyToMonitor, error)
	Get(ctx context.Context, publicKey domain.PublicKey) (domain.PublicKeyToMonitor, error)
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
	GetEvent              *GetEventHandler
	GetPublicKeyInfo      *GetPublicKeyInfoHandler
	GetEvents             *GetEventsHandler
}

type ReceivedEvent struct {
	relay domain.RelayAddress
	event domain.UnverifiedEvent
}

func NewReceivedEvent(relay domain.RelayAddress, event domain.UnverifiedEvent) ReceivedEvent {
	return ReceivedEvent{relay: relay, event: event}
}

func (r ReceivedEvent) Relay() domain.RelayAddress {
	return r.relay
}

func (r ReceivedEvent) Event() domain.UnverifiedEvent {
	return r.event
}

type ReceivedEventSubscriber interface {
	Subscribe(ctx context.Context) <-chan ReceivedEvent
}

type Metrics interface {
	StartApplicationCall(handlerName string) ApplicationCall
	ReportQueueLength(topic string, n int)
	ReportQueueOldestMessageAge(topic string, age time.Duration)
	ReportNumberOfStoredRelayAddresses(n int)
	ReportNumberOfStoredEvents(n int)
	ReportEventSentToRelay(address domain.RelayAddress, decision SendEventToRelayDecision, result SendEventToRelayResult)
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

var ErrEventSavedQueueEmpty = errors.New("event saved queue is empty")

type Subscriber interface {
	EventSavedQueueLength(ctx context.Context) (int, error)

	// EventSavedOldestMessageAge returns ErrEventSavedQueueEmpty.
	EventSavedOldestMessageAge(ctx context.Context) (time.Duration, error)
}

type EventSender interface {
	// SendEvent returns relays.ErrEventReplaced.
	SendEvent(ctx context.Context, relayAddress domain.RelayAddress, event domain.Event) error
}

var (
	SendEventToRelayDecisionSend   = SendEventToRelayDecision{"send"}
	SendEventToRelayDecisionIgnore = SendEventToRelayDecision{"ignore"}
)

type SendEventToRelayDecision struct {
	s string
}

func (r SendEventToRelayDecision) String() string {
	return r.s
}

var (
	SendEventToRelayResultIgnoreError = SendEventToRelayResult{"ignoreError"}
	SendEventToRelayResultSuccess     = SendEventToRelayResult{"success"}
	SendEventToRelayResultError       = SendEventToRelayResult{"error"}
)

type SendEventToRelayResult struct {
	s string
}

func (r SendEventToRelayResult) String() string {
	return r.s
}
