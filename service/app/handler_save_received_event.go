package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/bloom"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
)

var (
	saveFilter = NewEventFilter(
		internal.Pointer(MaxAgeLimit),
		nil,
		internal.Pointer(EventSizeLimit),
		internal.Pointer(TagNumberLimit),
	)
)

type SaveReceivedEvent struct {
	relay domain.RelayAddress
	event domain.UnverifiedEvent
}

func NewSaveReceivedEvent(relay domain.RelayAddress, event domain.UnverifiedEvent) SaveReceivedEvent {
	return SaveReceivedEvent{relay: relay, event: event}
}

type SaveReceivedEventHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
	duplicateFilter     *bloom.EventFilter
}

func NewSaveReceivedEventHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
	duplicateFilter *bloom.EventFilter,
) *SaveReceivedEventHandler {
	return &SaveReceivedEventHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("saveReceivedEventHandler"),
		metrics:             metrics,
		duplicateFilter:     duplicateFilter,
	}
}

// This handler is responsible for saving received events. It checks if the
// event should be saved and if so, it saves it and publishes the id to the
// internal db based event queue.
func (h *SaveReceivedEventHandler) Handle(ctx context.Context, cmd SaveReceivedEvent) (err error) {
	defer h.metrics.StartApplicationCall("saveReceivedEvent").End(&err)

	ctx, cancel := context.WithTimeout(ctx, applicationHandlerTimeout)
	defer cancel()

	h.logger.
		Trace().
		WithField("relay", cmd.relay.String()).
		WithField("event.id", cmd.event.Id().Hex()).
		WithField("event.createdAt", cmd.event.CreatedAt().String()).
		WithField("event.kind", cmd.event.Kind().Int()).
		Message("saving received event")

	if !saveFilter.IsOk(cmd.event) {
		return nil
	}

	// Quick check in Bloom filter first
	if h.duplicateFilter.Contains(cmd.event.Id()) {
		h.logger.
			Debug().
			WithField("event", cmd.event.String()).
			WithField("address", cmd.relay.String()).
			Message("event found in Bloom filter, skipping")
		h.metrics.ReportDuplicateEventBloomFilter()
		return nil
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		exists, err := adapters.Events.Exists(ctx, cmd.event.Id())
		if err != nil {
			return errors.Wrap(err, "error checking if event exists")
		}

		if exists {
			h.duplicateFilter.Add(cmd.event.Id()) // Add to filter since it exists
			return nil                            // we want to avoid publishing internal events for no reason
		}

		shouldBeDownloaded, err := h.shouldBeDownloaded(ctx, adapters, cmd.event)
		if err != nil {
			return errors.Wrap(err, "error checking if event should be downloaded")
		}

		if !shouldBeDownloaded {
			h.logger.
				Debug().
				WithField("event", cmd.event.String()).
				WithField("address", cmd.relay.String()).
				Message("event shouldn't have been downloaded, relay may be misbehaving")
			return nil
		}

		event, err := domain.NewEventFromUnverifiedEvent(cmd.event)
		if err != nil {
			return errors.Wrap(err, "error checking if event should be downloaded")
		}

		if err := adapters.Events.Save(ctx, event); err != nil {
			return errors.Wrap(err, "error saving the event")
		}

		h.duplicateFilter.Add(event.Id()) // Add to filter after successful save

		if err := adapters.Publisher.PublishEventSaved(ctx, event.Id()); err != nil {
			return errors.Wrap(err, "error publishing")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	return nil
}

func (h *SaveReceivedEventHandler) shouldBeDownloaded(ctx context.Context, adapters Adapters, event domain.UnverifiedEvent) (bool, error) {
	if downloader.IsGlobalEventKindToDownload(event.Kind()) {
		return true, nil
	}

	shouldBeDirectlyMonitored, err := h.shouldBeDirectlyMonitored(ctx, adapters, event)
	if err != nil {
		return false, errors.Wrap(err, "error checking if public key should be directly monitored")
	}

	if shouldBeDirectlyMonitored {
		return true, nil
	}

	isFolloweeOfMonitored, err := adapters.Contacts.IsFolloweeOfMonitoredPublicKey(ctx, event.PubKey())
	if err != nil {
		return false, errors.Wrap(err, "error checking if public key is a followee of a monitored key")
	}

	if isFolloweeOfMonitored {
		return true, nil
	}

	return false, nil
}

func (h *SaveReceivedEventHandler) shouldBeDirectlyMonitored(ctx context.Context, adapters Adapters, event domain.UnverifiedEvent) (bool, error) {
	if _, err := adapters.PublicKeysToMonitor.Get(ctx, event.PubKey()); err != nil {
		if errors.Is(err, ErrPublicKeyToMonitorNotFound) {
			return false, nil
		}
		return false, errors.Wrap(err, "error checking if public key to monitor exists")
	}

	return true, nil
}
