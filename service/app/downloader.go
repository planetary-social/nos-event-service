package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const (
	downloadEventsFromLast = 1 * time.Hour
	storeMetricsEvery      = 1 * time.Minute

	refreshKnownRelaysEvery     = 1 * time.Minute
	refreshKnownPublicKeysEvery = 1 * time.Minute
)

var (
	globalEventKindsToDownload = []domain.EventKind{
		domain.EventKindMetadata,
		domain.EventKindRecommendedRelay,
		domain.EventKindContacts,
		domain.EventKindRelayListMetadata,
	}
)

type ReceivedEventPublisher interface {
	Publish(relay domain.RelayAddress, event domain.UnverifiedEvent)
}

type BootstrapRelaySource interface {
	GetRelays(ctx context.Context) ([]domain.RelayAddress, error)
}

type RelaySource interface {
	GetRelays(ctx context.Context) ([]domain.RelayAddress, error)
}

type PublicKeySource interface {
	GetPublicKeys(ctx context.Context) ([]domain.PublicKey, error)
}

type Downloader struct {
	relayDownloaders     map[domain.RelayAddress]context.CancelFunc
	relayDownloadersLock sync.Mutex

	bootstrapRelaySource   BootstrapRelaySource
	relaySource            RelaySource
	logger                 logging.Logger
	metrics                Metrics
	relayDownloaderFactory *RelayDownloaderFactory
}

func NewDownloader(
	bootstrapRelaySource BootstrapRelaySource,
	relaySource RelaySource,
	logger logging.Logger,
	metrics Metrics,
	relayDownloaderFactory *RelayDownloaderFactory,
) *Downloader {
	return &Downloader{
		relayDownloaders: make(map[domain.RelayAddress]context.CancelFunc),

		bootstrapRelaySource:   bootstrapRelaySource,
		relaySource:            relaySource,
		logger:                 logger.New("downloader"),
		metrics:                metrics,
		relayDownloaderFactory: relayDownloaderFactory,
	}
}

func (d *Downloader) Run(ctx context.Context) error {
	go d.storeMetricsLoop(ctx)

	for {
		if err := d.updateDownloaders(ctx); err != nil {
			d.logger.
				Error().
				WithError(err).
				Message("error updating downloaders")
		}

		select {
		case <-time.After(refreshKnownRelaysEvery):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *Downloader) storeMetricsLoop(ctx context.Context) {
	for {
		d.storeMetrics()

		select {
		case <-time.After(storeMetricsEvery):
		case <-ctx.Done():
			return
		}
	}
}

func (d *Downloader) storeMetrics() {
	d.relayDownloadersLock.Lock()
	defer d.relayDownloadersLock.Unlock()

	d.metrics.ReportNumberOfRelayDownloaders(len(d.relayDownloaders))
}

func (d *Downloader) updateDownloaders(ctx context.Context) error {
	relays, err := d.getRelays(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting relays")
	}

	d.relayDownloadersLock.Lock()
	defer d.relayDownloadersLock.Unlock()

	for relayAddress, cancelFn := range d.relayDownloaders {
		if !relays.Contains(relayAddress) {
			d.logger.
				Trace().
				WithField("address", relayAddress.String()).
				Message("stopping a downloader")

			delete(d.relayDownloaders, relayAddress)
			cancelFn()
		}
	}

	for _, relayAddress := range relays.List() {
		if _, ok := d.relayDownloaders[relayAddress]; !ok {
			d.logger.
				Trace().
				WithField("address", relayAddress.String()).
				Message("creating a downloader")

			downloader, err := d.relayDownloaderFactory.CreateRelayDownloader(relayAddress)
			if err != nil {
				return errors.Wrap(err, "error creating a relay downloader")
			}

			ctx, cancel := context.WithCancel(ctx)
			go downloader.Run(ctx)
			d.relayDownloaders[relayAddress] = cancel
		}
	}

	return nil
}

func (d *Downloader) getRelays(ctx context.Context) (*internal.Set[domain.RelayAddress], error) {
	result := internal.NewEmptySet[domain.RelayAddress]()

	bootstrapRelays, err := d.bootstrapRelaySource.GetRelays(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting bootstrap relays")
	}
	result.PutMany(bootstrapRelays)

	databaseRelays, err := d.relaySource.GetRelays(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting relays")
	}
	result.PutMany(databaseRelays)

	return result, nil
}

type RelayDownloader struct {
	address domain.RelayAddress

	publicKeySubscriptionCancelFunc context.CancelFunc
	publicKeySubscriptionPublicKeys *internal.Set[domain.PublicKey]
	publicKeySubscriptionLock       sync.Mutex

	receivedEventPublisher ReceivedEventPublisher
	relayConnections       RelayConnections
	publicKeySource        PublicKeySource
	logger                 logging.Logger
	metrics                Metrics
}

func NewRelayDownloader(
	address domain.RelayAddress,
	receivedEventPublisher ReceivedEventPublisher,
	relayConnections RelayConnections,
	publicKeySource PublicKeySource,
	logger logging.Logger,
	metrics Metrics,
) *RelayDownloader {
	v := &RelayDownloader{
		address: address,

		publicKeySubscriptionPublicKeys: internal.NewEmptySet[domain.PublicKey](),

		receivedEventPublisher: receivedEventPublisher,
		relayConnections:       relayConnections,
		publicKeySource:        publicKeySource,
		logger:                 logger.New(fmt.Sprintf("relayDownloader(%s)", address.String())),
		metrics:                metrics,
	}
	return v
}

func (d *RelayDownloader) Run(ctx context.Context) {
	go d.downloadMessages(ctx, domain.NewFilter(
		nil,
		globalEventKindsToDownload,
		nil,
		d.downloadSince(),
	))

	for {
		if err := d.refreshPublicKeys(ctx); err != nil {
			d.logger.
				Error().
				WithError(err).
				Message("error refreshing public keys")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshKnownPublicKeysEvery):
			continue
		}
	}
}

func (d *RelayDownloader) downloadSince() *time.Time {
	return internal.Pointer(time.Now().Add(-downloadEventsFromLast))

}

func (d *RelayDownloader) refreshPublicKeys(ctx context.Context) error {
	publicKeys, err := d.publicKeySource.GetPublicKeys(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting public keys")
	}

	d.logger.
		Trace().
		WithField("n", len(publicKeys)).
		Message("got public keys")

	publicKeysSet := internal.NewSet(publicKeys)

	d.publicKeySubscriptionLock.Lock()
	defer d.publicKeySubscriptionLock.Unlock()

	if publicKeysSet.Equal(d.publicKeySubscriptionPublicKeys) {
		return nil
	}

	if d.publicKeySubscriptionCancelFunc != nil {
		d.publicKeySubscriptionCancelFunc()
	}

	ctx, cancel := context.WithCancel(ctx)
	go d.downloadMessages(ctx, domain.NewFilter(
		nil,
		nil,
		publicKeysSet.List(),
		d.downloadSince(),
	))
	d.publicKeySubscriptionPublicKeys = publicKeysSet
	d.publicKeySubscriptionCancelFunc = cancel

	return nil
}

func (d *RelayDownloader) downloadMessages(ctx context.Context, filter domain.Filter) {
	if err := d.downloadMessagesWithErr(ctx, filter); err != nil {
		d.logger.Error().WithError(err).Message("error downloading messages")
	}
}

func (d *RelayDownloader) downloadMessagesWithErr(ctx context.Context, filter domain.Filter) error {
	ch, err := d.relayConnections.GetEvents(ctx, d.address, filter)
	if err != nil {
		return errors.Wrap(err, "error getting events ch")
	}

	for eventOrEOSE := range ch {
		if !eventOrEOSE.EOSE() {
			d.metrics.ReportReceivedEvent(d.address)
			d.receivedEventPublisher.Publish(d.address, eventOrEOSE.Event())
		}
	}

	return nil
}

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

type RelayDownloaderFactory struct {
	publicKeySource        PublicKeySource
	relayConnections       RelayConnections
	receivedEventPublisher ReceivedEventPublisher
	logger                 logging.Logger
	metrics                Metrics
}

func NewRelayDownloaderFactory(
	publicKeySource PublicKeySource,
	relayConnections RelayConnections,
	receivedEventPublisher ReceivedEventPublisher,
	logger logging.Logger,
	metrics Metrics,
) *RelayDownloaderFactory {
	return &RelayDownloaderFactory{
		publicKeySource:        publicKeySource,
		relayConnections:       relayConnections,
		receivedEventPublisher: receivedEventPublisher,
		logger:                 logger.New("relayDownloaderFactory"),
		metrics:                metrics,
	}
}

func (r *RelayDownloaderFactory) CreateRelayDownloader(address domain.RelayAddress) (*RelayDownloader, error) {
	return NewRelayDownloader(
		address,
		r.receivedEventPublisher,
		r.relayConnections,
		r.publicKeySource,
		r.logger,
		r.metrics,
	), nil
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
