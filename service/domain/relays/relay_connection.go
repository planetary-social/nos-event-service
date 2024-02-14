package relays

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
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
	ReconnectionBackoff        = 5 * time.Minute
	MaxDialReconnectionBackoff = 30 * time.Minute
)

const (
	sendEventTimeout = 30 * time.Second
)

type Connection interface {
	// SendMessage can be called concurrently.
	SendMessage(msg NostrMessage) error

	// ReadMessage can't be called concurrently.
	ReadMessage() ([]byte, error)

	// Close is idempotent.
	Close() error
}

type NostrMessage interface {
	MarshalJSON() ([]byte, error)
}

type BackoffManager interface {
	GetReconnectionBackoff(err error) time.Duration
}

type ConnectionFactory interface {
	Create(ctx context.Context) (Connection, error)
	Address() domain.RelayAddress
}

type RelayConnection struct {
	connectionFactory ConnectionFactory
	logger            logging.Logger
	metrics           Metrics
	backoffManager    BackoffManager

	state      RelayConnectionState
	stateMutex sync.Mutex

	subscriptions          map[transport.SubscriptionID]subscription
	subscriptionsUpdatedCh chan struct{}
	subscriptionsMutex     sync.Mutex

	eventsToSend                  map[domain.EventId]*eventToSend
	eventsToSendMutex             sync.Mutex
	newEventsCh                   chan domain.Event
	rateLimitNoticeBackoffManager *RateLimitNoticeBackoffManager
}

func NewRelayConnection(
	connectionFactory ConnectionFactory,
	rateLimitNoticeBackoffManager *RateLimitNoticeBackoffManager,
	logger logging.Logger,
	metrics Metrics,
) *RelayConnection {
	return &RelayConnection{
		connectionFactory:             connectionFactory,
		logger:                        logger.New(fmt.Sprintf("relayConnection(%s)", connectionFactory.Address().String())),
		metrics:                       metrics,
		backoffManager:                NewDefaultBackoffManager(),
		state:                         RelayConnectionStateInitializing,
		subscriptions:                 make(map[transport.SubscriptionID]subscription),
		subscriptionsUpdatedCh:        make(chan struct{}),
		eventsToSend:                  make(map[domain.EventId]*eventToSend),
		newEventsCh:                   make(chan domain.Event),
		rateLimitNoticeBackoffManager: rateLimitNoticeBackoffManager,
	}
}

func (r *RelayConnection) Run(ctx context.Context) {
	for {
		err := r.run(ctx)
		if err != nil {
			r.logRunErr(err)
		}

		r.metrics.ReportRelayDisconnection(r.connectionFactory.Address(), err)

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
	return r.connectionFactory.Address()
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
		// Context is cancelled from a task.CheckIfDoneAndEnd() triggered from EOSE messages
		<-ctx.Done()
		if err := r.removeSubscriptionChannel(ch); err != nil {
			panic(err)
		}
	}()

	return ch, nil
}

// pushes the event to the eventsToSend channel and blocks until a sendEventResponse is received
func (r *RelayConnection) SendEvent(ctx context.Context, event domain.Event) error {
	ctx, cancel := context.WithTimeout(ctx, sendEventTimeout)
	defer cancel()

	ch := make(chan sendEventResponse)
	isFirstSchedule := r.scheduleSendingEvent(ctx, event, ch)

	defer func() {
		if err := r.removeEventChannel(event, ch); err != nil {
			panic(err)
		}
	}()

	if isFirstSchedule {
		select {
		case r.newEventsCh <- event:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

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

func (r *RelayConnection) scheduleSendingEvent(ctx context.Context, event domain.Event, ch chan sendEventResponse) bool {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	request := eventToSendRequest{
		ctx: ctx,
		ch:  ch,
	}

	v, ok := r.eventsToSend[event.Id()]
	if ok {
		v.requests = append(v.requests, request)
		return false
	} else {
		r.eventsToSend[event.Id()] = &eventToSend{
			event:    event,
			requests: []eventToSendRequest{request},
		}
		return true
	}
}

func (r *RelayConnection) logRunErr(err error) {
	l := r.logger.Error()
	if r.errorIsCommonAndShouldNotBeLoggedOnErrorLevel(err) {
		l = r.logger.Trace()
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

	return errors.New("somehow the channel was already removed?!")
}

func (r *RelayConnection) removeEventChannel(event domain.Event, chToRemove chan sendEventResponse) error {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	v, ok := r.eventsToSend[event.Id()]
	if !ok {
		return errors.New("this event wasn't scheduled to be sent?!")
	}

	for i, request := range v.requests {
		if chToRemove == request.ch {
			v.requests = append(v.requests[:i], v.requests[i+1:]...)
			if len(v.requests) == 0 {
				delete(r.eventsToSend, event.Id())
			}
			return nil
		}
	}

	return errors.New("somehow the channel was already removed?!")

}

func (r *RelayConnection) triggerSubscriptionUpdate() {
	select {
	case r.subscriptionsUpdatedCh <- struct{}{}:
	default:
	}
}

func (r *RelayConnection) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer r.setState(RelayConnectionStateDisconnected)

	r.logger.Trace().Message("connecting")

	conn, err := r.connectionFactory.Create(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating a connection")
	}

	r.setState(RelayConnectionStateConnected)
	r.logger.Trace().Message("connected")

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

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
		if err := r.sendEvents(ctx, conn); err != nil {
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

func parseEnvelope(messageBytes []byte) (nostr.Envelope, bool) {
	firstComma := bytes.Index(messageBytes, []byte{','})
	if firstComma == -1 {
		return nil, false
	}
	label := messageBytes[0:firstComma]
	containsClosed := bytes.Contains(label, []byte("CLOSED"))
	if containsClosed {
		return nil, true
	}

	nostrEnvelope := nostr.ParseMessage(messageBytes)
	return nostrEnvelope, false
}

type NoticeType string

const (
	NoticeTypeEmpty      NoticeType = "EMPTY"
	NoticeOnlyWithSearch NoticeType = "ONLY_WITH_SEARCH"
	NoticeRateLimit      NoticeType = "RATE_LIMIT"
	NoticeAuthRequired   NoticeType = "AUTH_REQUIRED"
	NoticeUnknownFeed    NoticeType = "UNKNOWN_FEED"
	NoticeInvalid        NoticeType = "INVALID"
	NoticeUnknownError   NoticeType = "UNKNOWN_ERROR"
	NoticeUnknown        NoticeType = "UNKNOWN"
)

var unknownFeedRegexp = regexp.MustCompile(`unknown.*feed`)
var rateLimitRegexp = regexp.MustCompile(`rate.*limit|too fast`)

func GetNoticeType(content string) NoticeType {
	content = strings.ToLower(content)
	content = strings.TrimSpace(content)

	switch {
	case content == "":
		return NoticeTypeEmpty
	case strings.HasSuffix(content, "only filter with search is supported"):
		return NoticeOnlyWithSearch
	case rateLimitRegexp.MatchString(content):
		return NoticeRateLimit
	case strings.Contains(content, "auth"):
		return NoticeAuthRequired
	case unknownFeedRegexp.MatchString(content):
		return NoticeUnknownFeed
	case strings.Contains(content, "invalid"):
		return NoticeInvalid
	case strings.Contains(content, "error"):
		return NoticeUnknownError
	default:
		return NoticeUnknown
	}
}

var everyTenSeconds func(func()) = logging.ConfigureTimeThrottler(10 * time.Second)

func (r *RelayConnection) handleMessage(messageBytes []byte) (err error) {
	address := r.connectionFactory.Address()

	envelope, isClosed := parseEnvelope(messageBytes)
	if envelope == nil && !isClosed {
		defer r.metrics.ReportMessageReceived(address, MessageTypeUnknown, &err)
		everyTenSeconds(func() {
			r.logger.
				Debug().
				WithField("message", string(messageBytes)).
				Message("received an unrecognized envelope message")
		})
		return errors.New("error parsing message, we are never going to find out what error unfortunately due to the design of this library")
	}

	switch v := envelope.(type) {
	case *nostr.EOSEEnvelope:
		defer r.metrics.ReportMessageReceived(address, MessageTypeEOSE, &err)
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
		defer r.metrics.ReportMessageReceived(address, MessageTypeEvent, &err)
		r.logger.
			Trace().
			WithField("subscription", *v.SubscriptionID).
			Message("received a message (event)")

		subscriptionID, err := transport.NewSubscriptionID(*v.SubscriptionID)
		if err != nil {
			return errors.Wrapf(err, "error creating subscription id from '%s'", *v.SubscriptionID)
		}

		event, err := domain.NewUnverifiedEvent(v.Event)
		if err != nil {
			return errors.Wrap(err, "error creating an event")
		}

		r.passEventOrEOSEToChannel(subscriptionID, NewEventOrEndOfSavedEventsWithEvent(event))
		return nil
	case *nostr.NoticeEnvelope:
		defer r.metrics.ReportMessageReceived(address, MessageTypeNotice, &err)
		noticeType := GetNoticeType(string(*v))

		r.logger.
			Debug().
			WithField("message", string(messageBytes)).
			WithField("noticeType", noticeType).
			Message("received a message (notice)")

		defer r.metrics.ReportNotice(address, noticeType)

		if noticeType == NoticeRateLimit {
			r.rateLimitNoticeBackoffManager.Bump()
		}
		return nil
	case *nostr.AuthEnvelope:
		defer r.metrics.ReportMessageReceived(address, MessageTypeAuth, &err)
		r.logger.
			Debug().
			WithField("message", string(messageBytes)).
			Message("received a message (auth)")
		return nil
	case *nostr.OKEnvelope:
		defer r.metrics.ReportMessageReceived(address, MessageTypeOK, &err)
		r.logger.
			Trace().
			WithField("message", string(messageBytes)).
			Message("received a message (ok)")

		eventID, err := domain.NewEventIdFromHex(v.EventID)
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
		if isClosed {
			defer r.metrics.ReportMessageReceived(address, MessageTypeClose, &err)
			r.logger.
				Debug().
				WithField("message", string(messageBytes)).
				Message("received a message (closed)")

			if r.rateLimitNoticeBackoffManager.rateLimitNoticeCount <= 0 {
				// Let's add a rate limit for close messages too, just in case some
				// relays close from rate limiting without sending a notice. It
				// makes also sense event if it's not for rate limit reasons but
				// just to avoid spamming relays that we know are closing often
				r.rateLimitNoticeBackoffManager.Bump()
			}
			return nil
		} else {
			defer r.metrics.ReportMessageReceived(address, MessageTypeUnknown, &err)

			// Add some traces to investigate these unknown messages
			everyTenSeconds(func() {
				r.logger.
					Debug().
					WithField("message", string(messageBytes)).
					Message("received an unknown message")
			})
			return errors.New("unknown message type")

		}
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

	v, ok := r.eventsToSend[eventID]
	if !ok {
		return
	}

	for _, request := range v.requests {
		select {
		case <-request.ctx.Done():
		case request.ch <- response:
		}
	}
}

func (r *RelayConnection) setState(state RelayConnectionState) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()
	r.state = state
}

func (r *RelayConnection) manageSubs(ctx context.Context, conn Connection) error {
	defer conn.Close()

	activeSubscriptions := internal.NewEmptySet[transport.SubscriptionID]()

	for {
		if err := r.updateSubs(conn, activeSubscriptions); err != nil {
			return errors.Wrap(err, "error updating subscriptions")
		}

		select {
		case <-r.subscriptionsUpdatedCh:
			// A subscription update has been triggered; continue to reevaluate subscriptions.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *RelayConnection) updateSubs(
	conn Connection,
	activeSubscriptions *internal.Set[transport.SubscriptionID],
) error {
	r.subscriptionsMutex.Lock()
	defer r.subscriptionsMutex.Unlock()

	for _, subscriptionID := range activeSubscriptions.List() {
		if _, ok := r.subscriptions[subscriptionID]; !ok {
			msg := transport.NewMessageClose(subscriptionID)

			r.logger.Trace().
				WithField("subscriptionID", subscriptionID).
				Message("closing subscription")

			r.rateLimitNoticeBackoffManager.Wait()
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

			r.rateLimitNoticeBackoffManager.Wait()
			if err := conn.SendMessage(msg); err != nil {
				return errors.Wrap(err, "writing req message error")
			}

			activeSubscriptions.Put(subscriptionID)
		}
	}

	r.metrics.ReportNumberOfSubscriptions(r.connectionFactory.Address(), len(r.subscriptions))
	return nil
}

func (r *RelayConnection) copyAllPastEvents() []domain.Event {
	r.eventsToSendMutex.Lock()
	defer r.eventsToSendMutex.Unlock()

	var result []domain.Event
	for _, eventToSent := range r.eventsToSend {
		result = append(result, eventToSent.event)
	}
	return result
}

func (r *RelayConnection) sendEvents(ctx context.Context, conn Connection) error {
	for _, event := range r.copyAllPastEvents() {
		msg := transport.NewMessageEvent(event)
		r.rateLimitNoticeBackoffManager.Wait()
		if err := conn.SendMessage(msg); err != nil {
			return errors.Wrap(err, "error writing a message")
		}
	}

	for {
		select {
		case event := <-r.newEventsCh:
			msg := transport.NewMessageEvent(event)
			r.rateLimitNoticeBackoffManager.Wait()
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
	event    domain.Event
	requests []eventToSendRequest
}

type eventToSendRequest struct {
	ctx context.Context
	ch  chan sendEventResponse
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

type WebsocketConnection struct {
	conn      *websocket.Conn
	writeLock sync.Mutex
	logger    logging.Logger
}

func NewWebsocketConnection(ctx context.Context, address domain.RelayAddress, logger logging.Logger) (*WebsocketConnection, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, address.String(), nil)
	if err != nil {
		return nil, NewDialError(err)
	}

	return &WebsocketConnection{
		conn:   conn,
		logger: logger.New(fmt.Sprintf("websocketConnection(%s)", address.String())),
	}, nil
}

func (c *WebsocketConnection) SendMessage(msg NostrMessage) error {
	messageBytes, err := msg.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "error marshaling message")
	}

	c.logger.
		Trace().
		WithField("message", string(messageBytes)).
		Message("sending a message")

	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if err := c.conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
		return errors.Wrap(err, "error writing message")
	}

	return nil
}

func (c *WebsocketConnection) ReadMessage() ([]byte, error) {
	_, messageBytes, err := c.conn.ReadMessage()
	if err != nil {
		return nil, NewReadMessageError(err)
	}

	c.logger.
		Trace().
		WithField("message", string(messageBytes)).
		Message("received a message")

	return messageBytes, nil
}

func (c *WebsocketConnection) Close() error {
	return c.conn.Close()
}

type WebsocketConnectionFactory struct {
	address domain.RelayAddress
	logger  logging.Logger
}

func NewWebsocketConnectionFactory(
	address domain.RelayAddress,
	logger logging.Logger,
) *WebsocketConnectionFactory {
	return &WebsocketConnectionFactory{
		address: address,
		logger:  logger,
	}
}

func (w *WebsocketConnectionFactory) Create(ctx context.Context) (Connection, error) {
	return NewWebsocketConnection(ctx, w.address, w.logger)
}

func (w *WebsocketConnectionFactory) Address() domain.RelayAddress {
	return w.address
}
