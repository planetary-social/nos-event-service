package app

import (
	"context"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
)

const cachePublicKeysFor = 1 * time.Minute

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
	start := time.Now()
	defer func() {
		m.logger.Debug().WithField("duration", time.Since(start)).Message("got relays")
	}()

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

func (d *DatabasePublicKeySource) GetPublicKeys(ctx context.Context) (downloader.PublicKeys, error) {
	start := time.Now()
	defer func() {
		d.logger.Debug().WithField("duration", time.Since(start)).Message("got public keys")
	}()

	publicKeysToMonitor := *internal.NewEmptySet[domain.PublicKey]()
	publicKeysToMonitorFollowees := *internal.NewEmptySet[domain.PublicKey]()

	if err := d.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		values, err := adapters.PublicKeysToMonitor.List(ctx)
		if err != nil {
			return errors.Wrap(err, "error getting public keys to monitor")
		}

		for _, value := range values {
			publicKeysToMonitor.Put(value.PublicKey())

			followees, err := adapters.Contacts.GetFollowees(ctx, value.PublicKey())
			if err != nil {
				return errors.Wrapf(err, "error getting followees of '%s", value.PublicKey().Hex())
			}
			publicKeysToMonitorFollowees.PutMany(followees)
		}

		return nil
	}); err != nil {
		return downloader.PublicKeys{}, errors.Wrap(err, "transaction error")
	}

	return downloader.NewPublicKeys(
		publicKeysToMonitor.List(),
		publicKeysToMonitorFollowees.List(),
	), nil
}

type CachedDatabasePublicKeySource struct {
	keys   *downloader.PublicKeys
	t      time.Time
	source downloader.PublicKeySource
}

func NewCachedDatabasePublicKeySource(
	source downloader.PublicKeySource,
) *CachedDatabasePublicKeySource {
	return &CachedDatabasePublicKeySource{
		source: source,
	}
}

func (d *CachedDatabasePublicKeySource) GetPublicKeys(ctx context.Context) (downloader.PublicKeys, error) {
	if d.keys == nil || time.Since(d.t) > cachePublicKeysFor {
		newKeys, err := d.source.GetPublicKeys(ctx)
		if err != nil {
			return downloader.PublicKeys{}, errors.Wrap(err, "error getting new public keys")
		}
		d.keys = &newKeys
		d.t = time.Now()
	}

	return *d.keys, nil
}
