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
	"github.com/planetary-social/nos-event-service/service/domain/relays"
)

const (
	downloadEventsFromLast  = 24 * time.Hour
	storeMetricsEvery       = 30 * time.Second
	refreshKnownRelaysEvery = 1 * time.Minute
)

type ReceivedEventPublisher interface {
	Publish(relay domain.RelayAddress, event domain.Event)
}

type BootstrapRelaySource interface {
	GetRelays(ctx context.Context) ([]domain.RelayAddress, error)
}

type RelaySource interface {
	GetRelays(ctx context.Context) ([]domain.RelayAddress, error)
}

type RelayConnections interface {
	GetEvents(ctx context.Context, relayAddress domain.RelayAddress, filter domain.Filter) (<-chan relays.EventOrEndOfSavedEvents, error)
}

type Downloader struct {
	relayDownloaders     map[domain.RelayAddress]context.CancelFunc
	relayDownloadersLock sync.Mutex

	bootstrapRelaySource   BootstrapRelaySource
	relaySource            RelaySource
	relayConnections       RelayConnections
	receivedEventPublisher ReceivedEventPublisher
	logger                 logging.Logger
	metrics                Metrics
}

func NewDownloader(
	bootstrapRelaySource BootstrapRelaySource,
	relaySource RelaySource,
	relayConnections RelayConnections,
	receivedEventPublisher ReceivedEventPublisher,
	logger logging.Logger,
	metrics Metrics,
) *Downloader {
	return &Downloader{
		relayDownloaders: make(map[domain.RelayAddress]context.CancelFunc),

		bootstrapRelaySource:   bootstrapRelaySource,
		relaySource:            relaySource,
		relayConnections:       relayConnections,
		receivedEventPublisher: receivedEventPublisher,
		logger:                 logger.New("downloader"),
		metrics:                metrics,
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

			downloader := NewRelayDownloader(
				d.receivedEventPublisher,
				d.relayConnections,
				d.logger,
				relayAddress,
			)

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

	//downloaders     map[relays.RelayAddress]context.CancelFunc
	//downloadersLock sync.Mutex

	receivedEventPublisher ReceivedEventPublisher
	relayConnections       RelayConnections
	logger                 logging.Logger
}

func NewRelayDownloader(
	receivedEventPublisher ReceivedEventPublisher,
	relayConnections RelayConnections,
	logger logging.Logger,
	address domain.RelayAddress,
) *RelayDownloader {
	v := &RelayDownloader{
		address: address,

		receivedEventPublisher: receivedEventPublisher,
		relayConnections:       relayConnections,
		logger:                 logger.New(fmt.Sprintf("relayDownloader(%s)", address.String())),
	}
	return v
}

func (d *RelayDownloader) Run(ctx context.Context) {
	//go d.storeMetricsLoop(ctx)

	go d.downloadMessages(ctx, domain.NewFilter(nil, d.eventKindsToDownloadForEveryone(), nil, d.downloadSince()))

	//for {
	//	if err := d.refreshRelays(ctx); err != nil {
	//		d.logger.Error().
	//			WithError(err).
	//			Message("error connecting and downloading")
	//	}
	//
	//	select {
	//	case <-ctx.Done():
	//		return
	//	case <-time.After(refreshPublicKeyDownloaderRelaysEvery):
	//		continue
	//	}
	//}
}

func (d *RelayDownloader) eventKindsToDownloadForEveryone() []domain.EventKind {
	return []domain.EventKind{
		domain.EventKindMetadata,
		domain.EventKindRecommendedRelay,
		domain.EventKindContacts,
		domain.EventKindRelayListMetadata,
	}
}

func (d *RelayDownloader) downloadSince() *time.Time {
	return internal.Pointer(time.Now().Add(-downloadEventsFromLast))

}

//func (d *RelayDownloader) storeMetricsLoop(ctx context.Context) {
//	for {
//		d.storeMetrics()
//
//		select {
//		case <-time.After(storeMetricsEvery):
//		case <-ctx.Done():
//			return
//		}
//	}
//}
//
//func (d *RelayDownloader) storeMetrics() {
//	d.downloadersLock.Lock()
//	defer d.downloadersLock.Unlock()
//
//	d.metrics.ReportNumberOfPublicKeyDownloaderRelays(d.publicKey, len(d.downloaders))
//}
//
//func (d *RelayDownloader) refreshRelays(ctx context.Context) error {
//	relayAddressesSet, err := d.getRelayAddresses(ctx)
//	if err != nil {
//		return errors.Wrap(err, "error getting relay addresses")
//	}
//
//	d.downloadersLock.Lock()
//	defer d.downloadersLock.Unlock()
//
//	for relayAddress, cancelFn := range d.downloaders {
//		if !relayAddressesSet.Contains(relayAddress) {
//			d.logger.Trace().
//				WithField("relayAddress", relayAddress.String()).
//				Message("stopping a relay downloader")
//			delete(d.downloaders, relayAddress)
//			cancelFn()
//		}
//	}
//
//	for _, relayAddress := range relayAddressesSet.List() {
//		if _, ok := d.downloaders[relayAddress]; !ok {
//			d.logger.Trace().
//				WithField("relayAddress", relayAddress.String()).
//				Message("creating a relay downloader")
//
//			ctx, cancel := context.WithCancel(ctx)
//			go d.downloadMessages(ctx, relayAddress)
//			d.downloaders[relayAddress] = cancel
//		}
//	}
//
//	return nil
//}
//
//func (d *RelayDownloader) getRelayAddresses(ctx context.Context) (*internal.Set[relays.RelayAddress], error) {
//	ctx, cancel := context.WithCancel(ctx)
//	defer cancel()
//
//	relayAddresses, err := d.relaySource.GetRelays(ctx, d.publicKey)
//	if err != nil {
//		return nil, errors.Wrap(err, "error getting relayAddresses")
//	}
//
//	d.logger.Debug().
//		WithField("numberOfAddresses", len(relayAddresses)).
//		WithField("publicKey", d.publicKey.Hex()).
//		Message("got relay addresses")
//
//	normalizedRelayAddresses := internal.NewEmptySet[relays.RelayAddress]()
//	for _, relayAddress := range relayAddresses {
//		normalizedRelayAddress, err := domain.NormalizeRelayAddress(relayAddress)
//		if err != nil {
//			return nil, errors.Wrapf(err, "error normalizing a relay address '%s'", relayAddress.String())
//		}
//		normalizedRelayAddresses.Put(normalizedRelayAddress)
//	}
//
//	return normalizedRelayAddresses, nil
//}

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
		result = append(result, address)
	}

	return result, nil
}
