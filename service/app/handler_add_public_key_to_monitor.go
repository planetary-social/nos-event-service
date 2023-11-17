package app

import (
	"context"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type AddPublicKeyToMonitor struct {
	publicKey domain.PublicKey
}

func NewAddPublicKeyToMonitor(publicKey domain.PublicKey) AddPublicKeyToMonitor {
	return AddPublicKeyToMonitor{publicKey: publicKey}
}

type AddPublicKeyToMonitorHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewAddPublicKeyToMonitorHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *AddPublicKeyToMonitorHandler {
	return &AddPublicKeyToMonitorHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("addPublicKeyToMonitorHandler"),
		metrics:             metrics,
	}
}

func (h *AddPublicKeyToMonitorHandler) Handle(ctx context.Context, cmd AddPublicKeyToMonitor) (err error) {
	defer h.metrics.StartApplicationCall("addPublicKeyToMonitor").End(&err)

	publicKeyToMonitor, err := domain.NewPublicKeyToMonitor(
		cmd.publicKey,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return errors.Wrap(err, "error creating a public key to monitor")
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		if err := adapters.PublicKeysToMonitor.Save(ctx, publicKeyToMonitor); err != nil {
			return errors.Wrap(err, "error saving the public key to monitor")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	return nil
}
