package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type GetPublicKeyInfo struct {
	publicKey domain.PublicKey
}

func NewGetPublicKeyInfo(publicKey domain.PublicKey) GetPublicKeyInfo {
	return GetPublicKeyInfo{publicKey: publicKey}
}

type PublicKeyInfo struct {
	numberOfFollowees int
	numberOfFollowers int
}

func NewPublicKeyInfo(numberOfFollowees int, numberOfFollowers int) PublicKeyInfo {
	return PublicKeyInfo{numberOfFollowees: numberOfFollowees, numberOfFollowers: numberOfFollowers}
}

func (p PublicKeyInfo) NumberOfFollowees() int {
	return p.numberOfFollowees
}

func (p PublicKeyInfo) NumberOfFollowers() int {
	return p.numberOfFollowers
}

type GetPublicKeyInfoHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewGetPublicKeyInfoHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *GetPublicKeyInfoHandler {
	return &GetPublicKeyInfoHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("getPublicKeyInfoHandler"),
		metrics:             metrics,
	}
}

func (h *GetPublicKeyInfoHandler) Handle(ctx context.Context, cmd GetPublicKeyInfo) (publicKeyInfo PublicKeyInfo, err error) {
	defer h.metrics.StartApplicationCall("getPublicKeyInfo").End(&err)

	ctx, cancel := context.WithTimeout(ctx, applicationHandlerTimeout)
	defer cancel()

	var followeesCount, followersCount int

	if err := h.transactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Contacts.CountFollowees(ctx, cmd.publicKey)
		if err != nil {
			return errors.Wrap(err, "error counting followees")
		}
		followeesCount = tmp

		tmp, err = adapters.Contacts.CountFollowers(ctx, cmd.publicKey)
		if err != nil {
			return errors.Wrap(err, "error counting followers")
		}
		followersCount = tmp

		return nil
	}); err != nil {
		return PublicKeyInfo{}, errors.Wrap(err, "transaction error")
	}

	return NewPublicKeyInfo(followeesCount, followersCount), nil
}
