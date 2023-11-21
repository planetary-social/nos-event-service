package relays

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/boreq/errors"
	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/oklog/ulid/v2"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	transport2 "github.com/planetary-social/nos-event-service/service/domain/relays/transport"
)

const (
	reconnectAfter = 1 * time.Minute
)

type RelayConnection struct {
	address domain.RelayAddress
	logger  logging.Logger
	metrics Metrics

	state      RelayConnectionState
	stateMutex sync.Mutex

	subscriptions                map[transport2.SubscriptionID]subscription
	subscriptionsUpdatedCh       chan struct{}
	subscriptionsUpdatedChClosed bool
	subscriptionsMutex           sync.Mutex
}

func NewRelayConnection(
	address domain.RelayAddress,
	logger logging.Logger,
	metrics Metrics,
) *RelayConnection {
	return &RelayConnection{
		address:                address,
		logger:                 logger.New(fmt.Sprintf("relayConnection(%s)", address.String())),
		metrics:                metrics,
		state:                  RelayConnectionStateInitializing,
		subscriptions:          make(map[transport2.SubscriptionID]subscription),
		subscriptionsUpdatedCh: make(chan struct{}),
	}
}

func (r *RelayConnection) Run(ctx context.Context) {
	for {
		if err := r.run(ctx); err != nil {
			l := r.logger.Error()
			if r.errorIsCommonAndShouldNotBeLoggedOnErrorLevel(err) {
				l = r.logger.Debug()
			}
			l.WithError(err).Message("encountered an error")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectAfter):
			continue
		}
	}
}

func (r *RelayConnection) errorIsCommonAndShouldNotBeLoggedOnErrorLevel(err error) bool {
	if errors.Is(err, DialError{}) {
		return true
	}

	if errors.Is(err, ReadMessageError{}) {
		return true
	}

	return false
}

func (r *RelayConnection) GetEvents(ctx context.Context, filter domain.Filter) (<-chan EventOrEndOfSavedEvents, error) {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	ch := make(chan EventOrEndOfSavedEvents)
	id, err := transport2.NewSubscriptionID(ulid.Make().String())
	if err != nil {
		return nil, errors.Wrap(err, "error creating a subscription id")
	}

	r.subscriptions[id] = subscription{
		ctx:    ctx,
		ch:     ch,
		id:     id,
		filter: filter,
	}

	r.triggerSubscriptionUpdate()

	go func() {
		<-ctx.Done()
		if err := r.removeChannel(ch); err != nil {
			panic(err)
		}
	}()

	return ch, nil
}

func (r *RelayConnection) State() RelayConnectionState {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()
	return r.state
}

func (r *RelayConnection) Address() domain.RelayAddress {
	return r.address
}

func (r *RelayConnection) removeChannel(chToRemove chan EventOrEndOfSavedEvents) error {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	for uuid, subscription := range r.subscriptions {
		if chToRemove == subscription.ch {
			close(subscription.ch)
			delete(r.subscriptions, uuid)
			r.triggerSubscriptionUpdate()
			return nil
		}
	}

	return errors.New("somehow the channel was already removed")
}

func (r *RelayConnection) triggerSubscriptionUpdate() {
	if !r.subscriptionsUpdatedChClosed {
		r.subscriptionsUpdatedChClosed = true
		close(r.subscriptionsUpdatedCh)
	}
}

func (r *RelayConnection) resetSubscriptionUpdateCh() {
	r.subscriptionsUpdatedChClosed = false
	r.subscriptionsUpdatedCh = make(chan struct{})
}

func (r *RelayConnection) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer r.setState(RelayConnectionStateDisconnected)

	r.logger.Trace().Message("connecting")
	r.metrics.ReportRelayReconnection(r.address)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, r.address.String(), nil)
	if err != nil {
		return NewDialError(err)
	}

	r.setState(RelayConnectionStateConnected)
	r.logger.Trace().Message("connected")

	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			r.logger.Debug().WithError(err).Message("error when closing connection due to closed context")
		}
	}()

	go func() {
		if err := r.manageSubs(ctx, conn); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, websocket.ErrCloseSent) {
				r.logger.Error().
					WithError(err).
					Message("error managing subs")
			}
		}
	}()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			return NewReadMessageError(err)
		}

		if err := r.handleMessage(messageBytes); err != nil {
			return errors.Wrap(err, "error handling message")
		}
	}
}

func (r *RelayConnection) handleMessage(messageBytes []byte) (err error) {
	envelope := nostr.ParseMessage(messageBytes)
	if envelope == nil {
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeUnknown, &err)
		r.logger.Error().
			WithField("message", string(messageBytes)).
			Message("error parsing an incoming message")
		return errors.New("error parsing message, we are never going to find out what error unfortunately due to the design of this library")
	}

	switch v := envelope.(type) {
	case *nostr.EOSEEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeEOSE, &err)
		r.logger.Trace().
			WithField("subscription", string(*v)).
			Message("received EOSE")

		subscriptionID, err := transport2.NewSubscriptionID(string(*v))
		if err != nil {
			return errors.Wrapf(err, "error creating subscription id from '%s'", string(*v))
		}
		r.passValueToChannel(subscriptionID, NewEventOrEndOfSavedEventsWithEOSE())
	case *nostr.EventEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeEvent, &err)
		r.logger.Trace().
			WithField("subscription", *v.SubscriptionID).
			Message("received event")

		subscriptionID, err := transport2.NewSubscriptionID(*v.SubscriptionID)
		if err != nil {
			return errors.Wrapf(err, "error creating subscription id from '%s'", *v.SubscriptionID)
		}
		event, err := domain.NewEvent(v.Event)
		if err != nil {
			return errors.Wrap(err, "error creating an event")
		}
		r.passValueToChannel(subscriptionID, NewEventOrEndOfSavedEventsWithEvent(event))
	case *nostr.NoticeEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeNotice, &err)
		r.logger.Debug().
			WithField("message", string(messageBytes)).
			Message("received a notice")
	default:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeUnknown, &err)
		r.logger.Debug().
			WithField("message", string(messageBytes)).
			Message("unhandled message")
	}

	return nil
}

func (r *RelayConnection) passValueToChannel(id transport2.SubscriptionID, value EventOrEndOfSavedEvents) {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	if sub, ok := r.subscriptions[id]; ok {
		select {
		case <-sub.ctx.Done():
		case sub.ch <- value:
		}
	}
}

func (r *RelayConnection) setState(state RelayConnectionState) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()
	r.state = state
}

func (r *RelayConnection) manageSubs(
	ctx context.Context,
	conn *websocket.Conn,
) error {
	defer conn.Close()

	activeSubscriptions := internal.NewEmptySet[transport2.SubscriptionID]()

	for {
		if err := r.updateSubs(conn, activeSubscriptions); err != nil {
			return errors.Wrap(err, "error updating subscriptions")
		}

		select {
		case <-r.subscriptionsUpdatedCh:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *RelayConnection) updateSubs(
	conn *websocket.Conn,
	activeSubscriptions *internal.Set[transport2.SubscriptionID],
) error {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	r.resetSubscriptionUpdateCh()

	for _, subscriptionID := range activeSubscriptions.List() {
		if _, ok := r.subscriptions[subscriptionID]; !ok {
			msg := transport2.NewMessageClose(subscriptionID)
			msgJSON, err := msg.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "marshaling close message failed")
			}

			r.logger.Trace().
				WithField("subscriptionID", subscriptionID).
				WithField("payload", string(msgJSON)).
				Message("closing subscription")

			if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				return errors.Wrap(err, "writing close message error")
			}

			activeSubscriptions.Delete(subscriptionID)
		}
	}

	for subscriptionID, subscription := range r.subscriptions {
		if ok := activeSubscriptions.Contains(subscriptionID); !ok {
			msg := transport2.NewMessageReq(subscription.id, []domain.Filter{subscription.filter})
			msgJSON, err := msg.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "marshaling req message failed")
			}

			r.logger.Trace().
				WithField("subscriptionID", subscriptionID).
				WithField("payload", string(msgJSON)).
				Message("opening subscription")

			if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				return errors.Wrap(err, "writing req message error")
			}

			activeSubscriptions.Put(subscriptionID)
		}
	}

	r.metrics.ReportNumberOfSubscriptions(r.address, len(r.subscriptions))
	return nil
}

type subscription struct {
	ctx context.Context
	ch  chan EventOrEndOfSavedEvents

	id     transport2.SubscriptionID
	filter domain.Filter
}

type DialError struct {
	underlying error
}

func NewDialError(underlying error) DialError {
	return DialError{underlying: underlying}
}

func (t DialError) Error() string {
	return fmt.Sprintf("error dialing the relay: %s", t.underlying)
}

func (t DialError) Unwrap() error {
	return t.underlying
}

func (t DialError) Is(target error) bool {
	_, ok1 := target.(DialError)
	_, ok2 := target.(*DialError)
	return ok1 || ok2
}

type ReadMessageError struct {
	underlying error
}

func NewReadMessageError(underlying error) ReadMessageError {
	return ReadMessageError{underlying: underlying}
}

func (t ReadMessageError) Error() string {
	return fmt.Sprintf("error reading a message from websocket: %s", t.underlying)
}

func (t ReadMessageError) Unwrap() error {
	return t.underlying
}

func (t ReadMessageError) Is(target error) bool {
	_, ok1 := target.(ReadMessageError)
	_, ok2 := target.(*ReadMessageError)
	return ok1 || ok2
}
