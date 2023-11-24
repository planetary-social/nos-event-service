package relays

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/boreq/errors"
	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/oklog/ulid/v2"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays/transport"
)

const (
	sendEventTimeout = 30 * time.Second
)

type BackoffManager interface {
	GetReconnectionBackoff(err error) time.Duration
}

type RelayConnection struct {
	address        domain.RelayAddress
	logger         logging.Logger
	metrics        Metrics
	backoffManager BackoffManager

	state      RelayConnectionState
	stateMutex sync.Mutex

	subscriptions                map[transport.SubscriptionID]subscription
	subscriptionsUpdatedCh       chan struct{}
	subscriptionsUpdatedChClosed bool
	subscriptionsMutex           sync.Mutex

	eventsToSend      map[domain.EventId][]eventToSend
	eventsToSendMutex sync.Mutex
	newEventsCh       chan domain.Event
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
		backoffManager:         NewDefaultBackoffManager(),
		state:                  RelayConnectionStateInitializing,
		subscriptions:          make(map[transport.SubscriptionID]subscription),
		subscriptionsUpdatedCh: make(chan struct{}),
		eventsToSend:           make(map[domain.EventId][]eventToSend),
		newEventsCh:            make(chan domain.Event),
	}
}

func (r *RelayConnection) Run(ctx context.Context) {
	for {
		err := r.run(ctx)
		if err != nil {
			r.logRunErr(err)
		}

		r.metrics.ReportRelayDisconnection(r.address, err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(r.backoffManager.GetReconnectionBackoff(err)):
			continue
		}
	}
}

func (r *RelayConnection) State() RelayConnectionState {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()
	return r.state
}

func (r *RelayConnection) Address() domain.RelayAddress {
	return r.address
}

func (r *RelayConnection) GetEvents(ctx context.Context, filter domain.Filter) (<-chan EventOrEndOfSavedEvents, error) {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	ch := make(chan EventOrEndOfSavedEvents)
	id, err := transport.NewSubscriptionID(ulid.Make().String())
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
		if err := r.removeSubscriptionChannel(ch); err != nil {
			panic(err)
		}
	}()

	return ch, nil
}

func (r *RelayConnection) SendEvent(ctx context.Context, event domain.Event) error {
	ctx, cancel := context.WithTimeout(ctx, sendEventTimeout)
	defer cancel()

	ch := make(chan sendEventResponse)
	r.scheduleSendingEvent(ctx, event, ch)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case response := <-ch:
		if !response.ok {
			return NewOKResponseError(response.message)
		}
		return nil
	}
}

func (r *RelayConnection) scheduleSendingEvent(ctx context.Context, event domain.Event, ch chan sendEventResponse) {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	r.eventsToSend[event.Id()] = append(r.eventsToSend[event.Id()], eventToSend{
		ctx:   ctx,
		ch:    ch,
		event: event,
	})

	go func() {
		<-ctx.Done()
		if err := r.removeEventChannel(event, ch); err != nil {
			panic(err)
		}
	}()

	go func() {
		select {
		case r.newEventsCh <- event:
		case <-ctx.Done():
		}
	}()
}

func (r *RelayConnection) logRunErr(err error) {
	l := r.logger.Error()
	if r.errorIsCommonAndShouldNotBeLoggedOnErrorLevel(err) {
		l = r.logger.Debug()
	}
	l.WithError(err).Message("encountered an error")
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

func (r *RelayConnection) removeSubscriptionChannel(chToRemove chan EventOrEndOfSavedEvents) error {
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

func (r *RelayConnection) removeEventChannel(event domain.Event, chToRemove chan sendEventResponse) error {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	for i, eventToSend := range r.eventsToSend[event.Id()] {
		if chToRemove == eventToSend.ch {
			r.eventsToSend[event.Id()] = append(r.eventsToSend[event.Id()][:i], r.eventsToSend[event.Id()][i+1:]...)
			if len(r.eventsToSend[event.Id()]) == 0 {
				delete(r.eventsToSend, event.Id())
			}
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

	conn, err := NewConnection(ctx, r.address)
	if err != nil {
		return errors.Wrap(err, "error creating a connection")
	}

	r.setState(RelayConnectionStateConnected)
	r.logger.Trace().Message("connected")

	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			r.logger.Debug().WithError(err).Message("error when closing connection due to closed context")
		}
	}()

	if err := r.queueResendingAllEvents(ctx); err != nil {
		return errors.Wrap(err, "error queueing events")
	}

	go func() {
		if err := r.manageSubs(ctx, conn); err != nil {
			if !r.writeErrorShouldNotBeLogged(err) {
				r.logger.Error().
					WithError(err).
					Message("error managing subs")
			}
		}
	}()

	go func() {
		if err := r.sendNewEvents(ctx, conn); err != nil {
			if !r.writeErrorShouldNotBeLogged(err) {
				r.logger.Error().
					WithError(err).
					Message("error resending events")
			}
		}
	}()

	for {
		messageBytes, err := conn.ReadMessage()
		if err != nil {
			return NewReadMessageError(err)
		}

		if err := r.handleMessage(messageBytes); err != nil {
			r.logger.
				Error().
				WithError(err).
				WithField("message", string(messageBytes)).
				Message("error handling an incoming message")
			continue
		}
	}
}

func (r *RelayConnection) writeErrorShouldNotBeLogged(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, websocket.ErrCloseSent)
}

func (r *RelayConnection) handleMessage(messageBytes []byte) (err error) {
	envelope := nostr.ParseMessage(messageBytes)
	if envelope == nil {
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeUnknown, &err)
		return errors.New("error parsing message, we are never going to find out what error unfortunately due to the design of this library")
	}

	switch v := envelope.(type) {
	case *nostr.EOSEEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeEOSE, &err)
		r.logger.
			Trace().
			WithField("subscription", string(*v)).
			Message("received a message (EOSE)")

		subscriptionID, err := transport.NewSubscriptionID(string(*v))
		if err != nil {
			return errors.Wrapf(err, "error creating subscription id from '%s'", string(*v))
		}
		r.passEventOrEOSEToChannel(subscriptionID, NewEventOrEndOfSavedEventsWithEOSE())
		return nil
	case *nostr.EventEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeEvent, &err)
		r.logger.
			Trace().
			WithField("subscription", *v.SubscriptionID).
			Message("received a message (event)")

		subscriptionID, err := transport.NewSubscriptionID(*v.SubscriptionID)
		if err != nil {
			return errors.Wrapf(err, "error creating subscription id from '%s'", *v.SubscriptionID)
		}

		event, err := domain.NewEvent(v.Event)
		if err != nil {
			return errors.Wrap(err, "error creating an event")
		}

		r.passEventOrEOSEToChannel(subscriptionID, NewEventOrEndOfSavedEventsWithEvent(event))
		return nil
	case *nostr.NoticeEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeNotice, &err)
		r.logger.
			Debug().
			WithField("message", string(messageBytes)).
			Message("received a message (notice)")
		return nil
	case *nostr.AuthEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeAuth, &err)
		r.logger.
			Debug().
			WithField("message", string(messageBytes)).
			Message("received a message (auth)")
		return nil
	case *nostr.OKEnvelope:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeOK, &err)
		r.logger.
			Trace().
			WithField("message", string(messageBytes)).
			Message("received a message (ok)")

		eventID, err := domain.NewEventId(v.EventID)
		if err != nil {
			return errors.Wrap(err, "error creating an event")
		}

		response := sendEventResponse{
			ok:      v.OK,
			message: v.Reason,
		}

		r.passSendEventResponseToChannel(eventID, response)
		return nil
	default:
		defer r.metrics.ReportMessageReceived(r.address, MessageTypeUnknown, &err)
		return errors.New("unknown message type")
	}
}

func (r *RelayConnection) passEventOrEOSEToChannel(id transport.SubscriptionID, value EventOrEndOfSavedEvents) {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	if sub, ok := r.subscriptions[id]; ok {
		select {
		case <-sub.ctx.Done():
		case sub.ch <- value:
		}
	}
}

func (r *RelayConnection) passSendEventResponseToChannel(eventID domain.EventId, response sendEventResponse) {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	for _, eventToSend := range r.eventsToSend[eventID] {
		select {
		case <-eventToSend.ctx.Done():
		case eventToSend.ch <- response:
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
	conn *Connection,
) error {
	defer conn.Close()

	activeSubscriptions := internal.NewEmptySet[transport.SubscriptionID]()

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
	conn *Connection,
	activeSubscriptions *internal.Set[transport.SubscriptionID],
) error {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	r.resetSubscriptionUpdateCh()

	for _, subscriptionID := range activeSubscriptions.List() {
		if _, ok := r.subscriptions[subscriptionID]; !ok {
			msg := transport.NewMessageClose(subscriptionID)

			r.logger.Trace().
				WithField("subscriptionID", subscriptionID).
				Message("closing subscription")

			if err := conn.SendMessage(msg); err != nil {
				return errors.Wrap(err, "writing close message error")
			}

			activeSubscriptions.Delete(subscriptionID)
		}
	}

	for subscriptionID, subscription := range r.subscriptions {
		if ok := activeSubscriptions.Contains(subscriptionID); !ok {
			msg := transport.NewMessageReq(subscription.id, []domain.Filter{subscription.filter})

			r.logger.Trace().
				WithField("subscriptionID", subscriptionID).
				Message("opening subscription")

			if err := conn.SendMessage(msg); err != nil {
				return errors.Wrap(err, "writing req message error")
			}

			activeSubscriptions.Put(subscriptionID)
		}
	}

	r.metrics.ReportNumberOfSubscriptions(r.address, len(r.subscriptions))
	return nil
}

func (r *RelayConnection) queueResendingAllEvents(ctx context.Context) error {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	for _, eventsToSend := range r.eventsToSend {
		if len(eventsToSend) == 0 {
			return errors.New("empty list which shouldn't happen as map entries get removed when list is empty")
		}

		event := eventsToSend[0].event
		go func() {
			select {
			case <-ctx.Done():
			case r.newEventsCh <- event:
			}
		}()
	}

	return nil
}

func (r *RelayConnection) sendNewEvents(ctx context.Context, conn *Connection) error {
	for {
		select {
		case event := <-r.newEventsCh:
			msg := transport.NewMessageEvent(event)
			if err := conn.SendMessage(msg); err != nil {
				return errors.Wrap(err, "error writing a message")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type subscription struct {
	ctx context.Context
	ch  chan EventOrEndOfSavedEvents

	id     transport.SubscriptionID
	filter domain.Filter
}

type eventToSend struct {
	ctx   context.Context
	ch    chan sendEventResponse
	event domain.Event
}

type sendEventResponse struct {
	ok      bool
	message string
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

type OKResponseError struct {
	reason string
}

func NewOKResponseError(reason string) OKResponseError {
	return OKResponseError{reason: reason}
}

func (t OKResponseError) Error() string {
	return fmt.Sprintf("received a false OK response from relay with reason '%s'", t.reason)
}

func (t OKResponseError) Is(target error) bool {
	_, ok1 := target.(OKResponseError)
	_, ok2 := target.(*OKResponseError)
	return ok1 || ok2
}

func (t OKResponseError) Reason() string {
	return t.reason
}

const (
	ReconnectionBackoff        = 5 * time.Minute
	MaxDialReconnectionBackoff = 30 * time.Minute
)

type DefaultBackoffManager struct {
	consecutiveDialErrors int
}

func NewDefaultBackoffManager() *DefaultBackoffManager {
	return &DefaultBackoffManager{}
}

func (d *DefaultBackoffManager) GetReconnectionBackoff(err error) time.Duration {
	if d.isDialError(err) {
		d.consecutiveDialErrors++
		return d.dialBackoff(d.consecutiveDialErrors)
	}

	d.consecutiveDialErrors = 0
	return ReconnectionBackoff
}

func (d *DefaultBackoffManager) dialBackoff(n int) time.Duration {
	a := time.Duration(math.Pow(5, float64(n))) * time.Second
	if a <= 0 {
		return MaxDialReconnectionBackoff
	}
	return min(a, MaxDialReconnectionBackoff)
}

func (d *DefaultBackoffManager) isDialError(err error) bool {
	return errors.Is(err, DialError{})
}

type NostrMessage interface {
	MarshalJSON() ([]byte, error)
}

type Connection struct {
	conn      *websocket.Conn
	writeLock sync.Mutex
}

func NewConnection(ctx context.Context, address domain.RelayAddress) (*Connection, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, address.String(), nil)
	if err != nil {
		return nil, NewDialError(err)
	}

	return &Connection{
		conn: conn,
	}, nil
}

func (c *Connection) SendMessage(msg NostrMessage) error {
	msgJSON, err := msg.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "error marshaling message")
	}

	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if err := c.conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
		return errors.Wrap(err, "error writing message")
	}

	return nil
}

func (c *Connection) ReadMessage() ([]byte, error) {
	_, messageBytes, err := c.conn.ReadMessage()
	if err != nil {
		return nil, NewReadMessageError(err)
	}

	return messageBytes, nil
}

func (c *Connection) Close() error {
	return c.conn.Close()
}
