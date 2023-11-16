package app

import (
	"context"
	"fmt"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type SaveReceivedEvent struct {
	relay domain.RelayAddress
	event domain.Event
}

func NewSaveReceivedEvent(relay domain.RelayAddress, event domain.Event) SaveReceivedEvent {
	return SaveReceivedEvent{relay: relay, event: event}
}

type SaveReceivedEventHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewSaveReceivedEventHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *SaveReceivedEventHandler {
	return &SaveReceivedEventHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("saveReceivedEventHandler"),
		metrics:             metrics,
	}
}

func (h *SaveReceivedEventHandler) Handle(ctx context.Context, cmd SaveReceivedEvent) (err error) {
	defer h.metrics.StartApplicationCall("saveReceivedEvent").End(&err)

	h.logger.
		Trace().
		WithField("relay", cmd.relay.String()).
		WithField("event.id", cmd.event.Id().Hex()).
		WithField("event.createdAt", cmd.event.CreatedAt().String()).
		WithField("event.kind", cmd.event.Kind().Int()).
		Message("saving received event")

	if !h.shouldBeGloballyDownloaded(cmd.event.Kind()) {
		return fmt.Errorf("event shouldn't have been downloaded, relay '%s' may be misbehaving", cmd.relay.String())
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		if err := adapters.Events.Save(ctx, cmd.event); err != nil {
			return errors.Wrap(err, "error saving the event")
		}

		if err := adapters.Publisher.PublishEventSaved(ctx, cmd.event.Id()); err != nil {
			return errors.Wrap(err, "error publishing")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	return nil
}

func (h *SaveReceivedEventHandler) shouldBeGloballyDownloaded(kind domain.EventKind) bool {
	for _, v := range globalEventTypesToDownload {
		if v == kind {
			return true
		}
	}
	return false
}
