package downloader

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
	storeMetricsEvery = 1 * time.Minute

	refreshKnownRelaysEvery = 1 * time.Minute
)

var (
	globalEventKindsToDownload = []domain.EventKind{
		domain.EventKindMetadata,
		domain.EventKindRecommendedRelay,
		domain.EventKindContacts,
		domain.EventKindRelayListMetadata,
	}
)

func IsGlobalEventKindToDownload(eventKind domain.EventKind) bool {
	for _, globalEventKind := range globalEventKindsToDownload {
		if globalEventKind == eventKind {
			return true
		}
	}
	return false
}

type Metrics interface {
	ReportNumberOfRelayDownloaders(n int)
	ReportReceivedEvent(address domain.RelayAddress)
}

type RelayConnections interface {
	GetEvents(ctx context.Context, relayAddress domain.RelayAddress, filter domain.Filter) (<-chan relays.EventOrEndOfSavedEvents, error)
}

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
	GetPublicKeys(ctx context.Context) (PublicKeys, error)
}

type PublicKeys struct {
	publicKeysToMonitor          *internal.Set[domain.PublicKey]
	publicKeysToMonitorFollowees *internal.Set[domain.PublicKey]
}

func NewPublicKeys(publicKeysToMonitor []domain.PublicKey, publicKeysToMonitorFollowees []domain.PublicKey) PublicKeys {
	return PublicKeys{
		publicKeysToMonitor:          internal.NewSet(publicKeysToMonitor),
		publicKeysToMonitorFollowees: internal.NewSet(publicKeysToMonitorFollowees),
	}
}

func (p PublicKeys) PublicKeysToMonitor() []domain.PublicKey {
	return p.publicKeysToMonitor.List()
}

func (p PublicKeys) PublicKeysToMonitorFollowees() []domain.PublicKey {
	return p.publicKeysToMonitorFollowees.List()
}

func (p PublicKeys) All() []domain.PublicKey {
	v := internal.NewEmptySet[domain.PublicKey]()
	v.PutMany(p.publicKeysToMonitor.List())
	v.PutMany(p.publicKeysToMonitorFollowees.List())
	return v.List()
}

func (p PublicKeys) Equal(o PublicKeys) bool {
	return p.publicKeysToMonitor.Equal(o.publicKeysToMonitor) &&
		p.publicKeysToMonitorFollowees.Equal(o.publicKeysToMonitorFollowees)
}

type Downloader struct {
	relayDownloaders     map[domain.RelayAddress]runningRelayDownloader
	previousPublicKeys   PublicKeys
	relayDownloadersLock sync.Mutex // protects relayDownloaders and previousPublicKeys

	bootstrapRelaySource   BootstrapRelaySource
	relaySource            RelaySource
	publicKeySource        PublicKeySource
	logger                 logging.Logger
	metrics                Metrics
	relayDownloaderFactory *RelayDownloaderFactory
}

func NewDownloader(
	bootstrapRelaySource BootstrapRelaySource,
	relaySource RelaySource,
	publicKeySource PublicKeySource,
	logger logging.Logger,
	metrics Metrics,
	relayDownloaderFactory *RelayDownloaderFactory,
) *Downloader {
	return &Downloader{
		relayDownloaders: make(map[domain.RelayAddress]runningRelayDownloader),

		bootstrapRelaySource:   bootstrapRelaySource,
		relaySource:            relaySource,
		publicKeySource:        publicKeySource,
		logger:                 logger.New("downloader"),
		metrics:                metrics,
		relayDownloaderFactory: relayDownloaderFactory,

		previousPublicKeys: NewPublicKeys(nil, nil),
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

	for relayAddress, runningRelayDownloader := range d.relayDownloaders {
		if !relays.Contains(relayAddress) {
			d.logger.
				Trace().
				WithField("address", relayAddress.String()).
				Message("stopping a downloader")

			delete(d.relayDownloaders, relayAddress)
			runningRelayDownloader.CancelFunc()
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
			if err := downloader.Start(ctx); err != nil {
				cancel()
				return errors.Wrap(err, "error starting a downloader")
			}
			d.relayDownloaders[relayAddress] = runningRelayDownloader{
				Context:         ctx,
				CancelFunc:      cancel,
				RelayDownloader: downloader,
			}
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

type runningRelayDownloader struct {
	Context         context.Context
	CancelFunc      context.CancelFunc
	RelayDownloader *RelayDownloader
}

type RelayDownloader struct {
	address                domain.RelayAddress
	scheduler              Scheduler
	receivedEventPublisher ReceivedEventPublisher
	relayConnections       RelayConnections
	logger                 logging.Logger
	metrics                Metrics
}

func NewRelayDownloader(
	address domain.RelayAddress,
	scheduler Scheduler,
	receivedEventPublisher ReceivedEventPublisher,
	relayConnections RelayConnections,
	logger logging.Logger,
	metrics Metrics,
) *RelayDownloader {
	v := &RelayDownloader{
		address:                address,
		scheduler:              scheduler,
		receivedEventPublisher: receivedEventPublisher,
		relayConnections:       relayConnections,
		logger:                 logger.New(fmt.Sprintf("relayDownloader(%s)", address.String())),
		metrics:                metrics,
	}
	return v
}

func (d *RelayDownloader) Start(ctx context.Context) error {
	ch, err := d.scheduler.GetTasks(ctx, d.address)
	if err != nil {
		return errors.Wrap(err, "error getting task channel")
	}

	go func() {
		for task := range ch {
			d.performTask(task)
		}
	}()

	return nil
}

func (d *RelayDownloader) performTask(task Task) {
	if err := d.performTaskWithErr(task); err != nil {
		d.logger.Error().WithError(err).Message("error downloading messages")
		task.OnError(err)
	}
}

func (d *RelayDownloader) performTaskWithErr(task Task) error {
	ch, err := d.relayConnections.GetEvents(task.Ctx(), d.address, task.Filter())
	if err != nil {
		return errors.Wrap(err, "error getting events ch")
	}

	for eventOrEOSE := range ch {
		if eventOrEOSE.EOSE() {
			task.OnReceivedEOSE()
		} else {
			d.metrics.ReportReceivedEvent(d.address)
			d.receivedEventPublisher.Publish(d.address, eventOrEOSE.Event())
		}
	}

	return nil
}

type RelayDownloaderFactory struct {
	relayConnections       RelayConnections
	receivedEventPublisher ReceivedEventPublisher
	scheduler              Scheduler
	logger                 logging.Logger
	metrics                Metrics
}

func NewRelayDownloaderFactory(
	relayConnections RelayConnections,
	receivedEventPublisher ReceivedEventPublisher,
	scheduler Scheduler,
	logger logging.Logger,
	metrics Metrics,
) *RelayDownloaderFactory {
	return &RelayDownloaderFactory{
		relayConnections:       relayConnections,
		receivedEventPublisher: receivedEventPublisher,
		scheduler:              scheduler,
		logger:                 logger.New("relayDownloaderFactory"),
		metrics:                metrics,
	}
}

func (r *RelayDownloaderFactory) CreateRelayDownloader(address domain.RelayAddress) (*RelayDownloader, error) {
	return NewRelayDownloader(
		address,
		r.scheduler,
		r.receivedEventPublisher,
		r.relayConnections,
		r.logger,
		r.metrics,
	), nil
}
