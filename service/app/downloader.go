package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type DatabaseRelaySource struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
}

func NewDatabaseRelaySource(transactionProvider TransactionProvider, logger logging.Logger) *DatabaseRelaySource {
	return &DatabaseRelaySource{
		transactionProvider: transactionProvider,
		logger:              logger.New("databaseRelaySource"),
	}
}

func (m *DatabaseRelaySource) GetRelays(ctx context.Context) ([]domain.RelayAddress, error) {
	var maybeResult []domain.MaybeRelayAddress
	if err := m.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Relays.List(ctx)
		if err != nil {
			return errors.Wrap(err, "error listing relays")
		}
		maybeResult = tmp
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "transaction error")
	}

	var result []domain.RelayAddress
	for _, maybe := range maybeResult {
		address, err := domain.NewRelayAddressFromMaybeAddress(maybe)
		if err != nil {
			m.logger.
				Debug().
				WithError(err).
				WithField("address", maybe.String()).
				Message("address is invalid")
			continue
		}

		if address.IsLoopbackOrPrivate() {
			continue
		}

		result = append(result, address)
	}

	return result, nil
}

type DatabasePublicKeySource struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
}

func NewDatabasePublicKeySource(transactionProvider TransactionProvider, logger logging.Logger) *DatabasePublicKeySource {
	return &DatabasePublicKeySource{
		transactionProvider: transactionProvider,
		logger:              logger.New("databasePublicKeySource"),
	}
}

func (d *DatabasePublicKeySource) GetPublicKeys(ctx context.Context) ([]domain.PublicKey, error) {
	result := internal.NewEmptySet[domain.PublicKey]()

	if err := d.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		publicKeysToMonitor, err := adapters.PublicKeysToMonitor.List(ctx)
		if err != nil {
			return errors.Wrap(err, "error getting public keys to monitor")
		}

		for _, v := range publicKeysToMonitor {
			result.Put(v.PublicKey())

			followees, err := adapters.Contacts.GetFollowees(ctx, v.PublicKey())
			if err != nil {
				return errors.Wrapf(err, "error getting followees of '%s", v.PublicKey().Hex())
			}

			result.PutMany(followees)
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "transaction error")
	}

	return result.List(), nil
}
