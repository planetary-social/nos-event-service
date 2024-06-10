package relays

import (
	"context"
	"sync"
	"time"

	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type Metrics interface {
	ReportRelayConnectionsState(m map[domain.RelayAddress]RelayConnectionState)
	ReportNumberOfSubscriptions(address domain.RelayAddress, n int)
	ReportRateLimitBackoffMs(address domain.RelayAddress, n int)
	ReportMessageReceived(address domain.RelayAddress, messageType MessageType, err *error)
	ReportRelayDisconnection(address domain.RelayAddress, err error)
	ReportNotice(address domain.RelayAddress, noticeType NoticeType)
}

type MessageType struct {
	s string
}

func (t MessageType) String() string {
	return t.s
}

var (
	MessageTypeNotice  = MessageType{"notice"}
	MessageTypeEOSE    = MessageType{"eose"}
	MessageTypeEvent   = MessageType{"event"}
	MessageTypeAuth    = MessageType{"auth"}
	MessageTypeOK      = MessageType{"ok"}
	MessageTypeClose   = MessageType{"close"}
	MessageTypeUnknown = MessageType{"unknown"}
)

const (
	storeMetricsEvery = 30 * time.Second
)

type RelayConnections struct {
	logger  logging.Logger
	metrics Metrics

	longCtx context.Context

	connections                    map[domain.RelayAddress]*RelayConnection
	rateLimitNoticeBackoffManagers map[string]*RateLimitNoticeBackoffManager
	connectionsLock                sync.Mutex
}

func NewRelayConnections(ctx context.Context, logger logging.Logger, metrics Metrics) *RelayConnections {
	v := &RelayConnections{
		logger:                         logger.New("relayConnections"),
		metrics:                        metrics,
		longCtx:                        ctx,
		connections:                    make(map[domain.RelayAddress]*RelayConnection),
		rateLimitNoticeBackoffManagers: make(map[string]*RateLimitNoticeBackoffManager),
		connectionsLock:                sync.Mutex{},
	}
	go v.storeMetricsLoop(ctx)
	return v
}

func (d *RelayConnections) GetEvents(ctx context.Context, relayAddress domain.RelayAddress, filter domain.Filter) (<-chan EventOrEndOfSavedEvents, error) {
	connection := d.getConnection(relayAddress)
	return connection.GetEvents(ctx, filter)
}

func (d *RelayConnections) SendEvent(ctx context.Context, relayAddress domain.RelayAddress, event domain.Event) error {
	connection := d.getConnection(relayAddress)
	return connection.SendEvent(ctx, event)
}

func (d *RelayConnections) NotifyBackPressure() {
	d.connectionsLock.Lock()
	defer d.connectionsLock.Unlock()
	for _, connection := range d.connections {
		if connection.cancelRun != nil && connection.Address().HostWithoutPort() != "relay.nos.social" {
			connection.cancelRun()
			connection.cancelRun = nil
		}
	}
}

func (d *RelayConnections) ResolveBackPressure() {
	d.connectionsLock.Lock()
	defer d.connectionsLock.Unlock()
	for _, connection := range d.connections {
		if connection.cancelBackPressure != nil {
			connection.cancelBackPressure()
			connection.cancelBackPressure = nil
		}
	}
}

func (d *RelayConnections) storeMetricsLoop(ctx context.Context) {
	for {
		d.storeMetrics()

		select {
		case <-time.After(storeMetricsEvery):
		case <-ctx.Done():
			return
		}
	}
}

func (d *RelayConnections) storeMetrics() {
	d.connectionsLock.Lock()
	defer d.connectionsLock.Unlock()

	m := make(map[domain.RelayAddress]RelayConnectionState)
	for _, connection := range d.connections {
		rateLimitNoticeBackoffManager := d.getRateLimitNoticeBackoffManager(connection.Address())
		d.metrics.ReportRateLimitBackoffMs(connection.Address(), rateLimitNoticeBackoffManager.GetBackoffMs())
		m[connection.Address()] = connection.State()
	}
	d.metrics.ReportRelayConnectionsState(m)
}
func (r *RelayConnections) getRateLimitNoticeBackoffManager(relayAddress domain.RelayAddress) *RateLimitNoticeBackoffManager {
	rateLimitNoticeBackoffManager, exists := r.rateLimitNoticeBackoffManagers[relayAddress.HostWithoutPort()]
	if !exists {
		rateLimitNoticeBackoffManager = NewRateLimitNoticeBackoffManager()
		r.rateLimitNoticeBackoffManagers[relayAddress.HostWithoutPort()] = rateLimitNoticeBackoffManager
	}

	return rateLimitNoticeBackoffManager
}

// Notice that a single connection can serve multiple req. This can cause a too many concurrent requests error if not throttled.
func (r *RelayConnections) getConnection(relayAddress domain.RelayAddress) *RelayConnection {
	r.connectionsLock.Lock()
	defer r.connectionsLock.Unlock()

	if connection, ok := r.connections[relayAddress]; ok {
		return connection
	}

	factory := NewWebsocketConnectionFactory(relayAddress, r.logger)

	// Sometimes different addreses can point to the same relay. Example is
	// wss://feeds.nostr.band/video and wss://feeds.nostr.band/audio.  For these
	// cases, we want to share the rate limit notice backoff manager.
	rateLimitNoticeBackoffManager := r.getRateLimitNoticeBackoffManager(relayAddress)

	connection := NewRelayConnection(factory, rateLimitNoticeBackoffManager, r.logger, r.metrics)
	go connection.Run(r.longCtx)

	r.connections[relayAddress] = connection
	return connection
}
