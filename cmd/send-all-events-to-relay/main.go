package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
	"github.com/sirupsen/logrus"
)

var (
	relayAddress = domain.MustNewRelayAddress("wss://relay.nos.social")
)

const (
	serviceAddress  = "https://events.nos.social"
	eventChanBuffer = 500
	numWorkers      = 50
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	logger := newLogger()
	metrics := newMockMetrics()
	d := NewEventDownloader(serviceAddress, logger)
	connections := relays.NewRelayConnections(ctx, logger, metrics)
	sender := relays.NewEventSender(connections)

	ch := d.Download(ctx)
	u := NewEventUploader(ctx, sender, relayAddress, logger, ch)

	if err := u.Wait(); err != nil {
		return errors.Wrap(err, "error waiting")
	}

	return nil
}

type EventUploader struct {
	address                  domain.RelayAddress
	eventSender              *relays.EventSender
	logger                   logging.Logger
	eventsRelayReplaced      atomic.Int64
	eventsRelayDidNotReplace atomic.Int64
	allEvents                atomic.Int64
	eventCh                  <-chan EventOrError
	errCh                    chan error
}

func NewEventUploader(ctx context.Context, eventSender *relays.EventSender, address domain.RelayAddress, logger logging.Logger, ch <-chan EventOrError) *EventUploader {
	u := &EventUploader{
		address:     address,
		eventSender: eventSender,
		logger:      logger,
		eventCh:     ch,
		errCh:       make(chan error),
	}
	u.startWorkers(ctx)
	return u
}

func (u *EventUploader) Wait() error {
	for i := 0; i < numWorkers; i++ {
		if err := <-u.errCh; err != nil {
			return err
		}
	}
	return nil
}

func (u *EventUploader) startWorkers(ctx context.Context) {
	for i := 0; i < numWorkers; i++ {
		go u.worker(ctx)
	}
}

func (u *EventUploader) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case unverifiedEventOrError, ok := <-u.eventCh:
			if !ok {
				select {
				case <-ctx.Done():
					return
				case u.errCh <- nil:
					return
				}
			}

			if err := unverifiedEventOrError.Err(); err != nil {
				select {
				case <-ctx.Done():
					return
				case u.errCh <- errors.Wrap(err, "received an error"):
					return
				}
			}

			unverifiedEvent := unverifiedEventOrError.Event()

			if !app.ShouldSendEventToRelay(unverifiedEvent) {
				continue
			}

			event, err := domain.NewEventFromUnverifiedEvent(unverifiedEvent)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case u.errCh <- errors.Wrap(err, "error verifying the event"):
					return
				}
			}

			if err := u.sendEvent(ctx, event); err != nil {
				select {
				case <-ctx.Done():
					return
				case u.errCh <- errors.Wrap(err, "error sending the event"):
					return
				}
			}
		}
	}
}

func (u *EventUploader) sendEvent(ctx context.Context, event domain.Event) error {
	for {
		if err := u.eventSender.SendEvent(ctx, u.address, event); err != nil {
			if errors.Is(err, relays.BackPressureError) {
				u.eventsRelayReplaced.Add(1)
				u.allEvents.Add(1)
			} else {
				u.logger.Error().WithError(err).Message("error sending event, maybe retrying")

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}
		} else {
			u.eventsRelayDidNotReplace.Add(1)
			u.allEvents.Add(1)
		}

		if v := u.allEvents.Load(); v%100 == 0 {
			u.logger.Debug().
				WithField("all", v).
				WithField("notReplaced", u.eventsRelayDidNotReplace.Load()).
				WithField("replaced", u.eventsRelayReplaced.Load()).
				Message("processed events")
		}

		return nil
	}
}

type EventDownloader struct {
	serviceAddress string
	client         http.Client
	logger         logging.Logger
}

func NewEventDownloader(serviceAddress string, logger logging.Logger) *EventDownloader {
	return &EventDownloader{
		serviceAddress: serviceAddress,
		client: http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (d *EventDownloader) Download(ctx context.Context) <-chan EventOrError {
	ch := make(chan EventOrError, eventChanBuffer)
	go d.download(ctx, ch)
	return ch
}

func (d *EventDownloader) download(ctx context.Context, ch chan EventOrError) {
	defer close(ch)

	var after *domain.EventId

	counter := 0
	for {
		events, hasMoreEvents, err := d.list(ctx, after)
		if err != nil {
			err = errors.Wrap(err, "error listing events")
			select {
			case <-ctx.Done():
			case ch <- NewEventOrErrorWithError(err):
			}
			return
		}

		for _, event := range events {
			select {
			case <-ctx.Done():
			case ch <- NewEventOrErrorWithEvent(event):
			}
		}

		if !hasMoreEvents {
			return
		}

		if len(events) == 0 {
			select {
			case <-ctx.Done():
			case ch <- NewEventOrErrorWithError(errors.New("something went wrong as I can't select a new 'after'")):
			}
		}

		counter += len(events)
		lastEvent := events[len(events)-1]
		after = internal.Pointer(lastEvent.Id())
		d.logger.Debug().
			WithField("after", after.Hex()).
			WithField("totalEvents", counter).
			Message("selected a new 'after'")
	}
}

func (d *EventDownloader) list(ctx context.Context, after *domain.EventId) ([]domain.UnverifiedEvent, bool, error) {
	url := fmt.Sprintf("%s/events", strings.TrimRight(serviceAddress, "/"))

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, errors.Wrap(err, "error creating a request")
	}

	if after != nil {
		q := request.URL.Query()
		q.Add("after", after.Hex())
		request.URL.RawQuery = q.Encode()
	}

	response, err := d.client.Do(request)
	if err != nil {
		return nil, false, errors.Wrap(err, "error performing a request")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("got status '%d'", response.StatusCode)
	}

	var transport listEventsResponse
	if err := json.NewDecoder(response.Body).Decode(&transport); err != nil {
		return nil, false, errors.Wrap(err, "error decoding a response")
	}

	var result []domain.UnverifiedEvent
	for _, rawEvent := range transport.Events {
		event, err := domain.NewUnverifiedEventFromRaw(rawEvent)
		if err != nil {
			return nil, false, errors.Wrap(err, "error creating an event")
		}
		result = append(result, event)
	}
	return result, transport.ThereIsMoreEvents, nil
}

type listEventsResponse struct {
	Events            []json.RawMessage `json:"events"`
	ThereIsMoreEvents bool              `json:"thereIsMoreEvents"`
}

type EventOrError struct {
	event domain.UnverifiedEvent
	err   error
}

func NewEventOrErrorWithEvent(event domain.UnverifiedEvent) EventOrError {
	return EventOrError{
		event: event,
	}
}

func NewEventOrErrorWithError(err error) EventOrError {
	return EventOrError{
		err: err,
	}
}

func (e *EventOrError) Event() domain.UnverifiedEvent {
	return e.event
}

func (e *EventOrError) Err() error {
	return e.err
}

func newLogger() logging.Logger {
	v := logrus.New()
	v.SetLevel(logrus.DebugLevel)
	return logging.NewSystemLogger(logging.NewLogrusLoggingSystem(v), "root")
}

type mockMetrics struct {
}

func newMockMetrics() mockMetrics {
	return mockMetrics{}
}

func (m mockMetrics) ReportRelayConnectionsState(v map[domain.RelayAddress]relays.RelayConnectionState) {
}

func (m mockMetrics) ReportNumberOfSubscriptions(address domain.RelayAddress, n int) {
}

func (m mockMetrics) ReportRateLimitBackoffMs(address domain.RelayAddress, n int) {
}

func (m mockMetrics) ReportMessageReceived(address domain.RelayAddress, messageType relays.MessageType, err *error) {
}

func (m mockMetrics) ReportRelayDisconnection(address domain.RelayAddress, err error) {
}

func (m mockMetrics) ReportNotice(address domain.RelayAddress, noticeType relays.NoticeType) {
}
