package prometheus

import (
	"runtime/debug"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	labelHandlerName = "handlerName"
	labelTopic       = "topic"
	labelVcsRevision = "vcsRevision"
	labelVcsTime     = "vcsTime"
	labelGo          = "go"

	labelAddress         = "address"
	labelConnectionState = "state"

	labelMessageType = "messageType"

	labelResult                     = "result"
	labelResultSuccess              = "success"
	labelResultError                = "error"
	labelResultInvalidPointerPassed = "invalidPointerPassed"

	labelReason = "reason"

	labelDecision   = "decision"
	labelNoticeType = "noticeType"
)

type Prometheus struct {
	noticeTypeCounter                       *prometheus.CounterVec
	applicationHandlerCallsCounter          *prometheus.CounterVec
	applicationHandlerCallDurationHistogram *prometheus.HistogramVec
	relayDownloadersGauge                   prometheus.Gauge
	subscriptionQueueLengthGauge            *prometheus.GaugeVec
	subscriptionQueueOldestMessageAgeGauge  *prometheus.GaugeVec
	relayConnectionStateGauge               *prometheus.GaugeVec
	receivedEventsCounter                   *prometheus.CounterVec
	relayConnectionSubscriptionsGauge       *prometheus.GaugeVec
	relayRateLimitBackoffMsGauge            *prometheus.GaugeVec
	relayConnectionReceivedMessagesCounter  *prometheus.CounterVec
	relayConnectionDisconnectionsCounter    *prometheus.CounterVec
	storedRelayAddressesGauge               prometheus.Gauge
	storedEventsGauge                       prometheus.Gauge
	eventsSentToRelayCounter                *prometheus.CounterVec

	registry *prometheus.Registry

	logger logging.Logger
}

func NewPrometheus(logger logging.Logger) (*Prometheus, error) {
	noticeTypeCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notice_type_total",
			Help: "Total number of notices per notice type.",
		},
		[]string{labelAddress, labelNoticeType},
	)
	applicationHandlerCallsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "application_handler_calls_total",
			Help: "Total number of calls to application handlers.",
		},
		[]string{labelHandlerName, labelResult},
	)
	applicationHandlerCallDurationHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "application_handler_calls_duration",
			Help: "Duration of calls to application handlers in seconds.",
		},
		[]string{labelHandlerName, labelResult},
	)
	versionGague := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "version",
			Help: "This metric exists just to put a commit label on it.",
		},
		[]string{labelVcsRevision, labelVcsTime, labelGo},
	)
	subscriptionQueueLengthGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "subscription_queue_length",
			Help: "Number of events in the subscription queue.",
		},
		[]string{labelTopic},
	)
	subscriptionQueueOldestMessageAgeGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "subscription_queue_oldest_message_age",
			Help: "Age of the oldest event in the subscription queue in seconds.",
		},
		[]string{labelTopic},
	)
	relayDownloadersGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "relay_downloader_gauge",
			Help: "Number of running relay downloaders.",
		},
	)
	relayConnectionStateGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_connection_state_gauge",
			Help: "State of relay connections.",
		},
		[]string{labelAddress, labelConnectionState},
	)
	receivedEventsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "received_events_counter",
			Help: "Number of received events.",
		},
		[]string{labelAddress},
	)
	relayConnectionSubscriptionsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_connection_subscriptions_gauge",
			Help: "Number of relay connection subscriptions.",
		},
		[]string{labelAddress},
	)
	relayRateLimitBackoffMsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "relay_rate_limit_backoff_ms_gauge",
			Help: "Rate limit wait in milliseconds.",
		},
		[]string{labelAddress},
	)
	relayConnectionReceivedMessagesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_connection_received_messages_counter",
			Help: "Number of received messages and their processing result.",
		},
		[]string{labelAddress, labelMessageType, labelResult},
	)
	relayConnectionDisconnectionsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_connection_disconnections_counter",
			Help: "Number of disconnections per relay.",
		},
		[]string{labelAddress, labelReason},
	)
	storedRelayAddressesGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "stored_relay_addresses_gauge",
			Help: "Number of stored relay addresses.",
		},
	)
	storedEventsGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "stored_events_gauge",
			Help: "Number of stored events.",
		},
	)
	eventsSentToRelayCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "events_sent_to_relay_counter",
			Help: "Number of events sent to relay.",
		},
		[]string{labelAddress, labelDecision, labelResult},
	)

	reg := prometheus.NewRegistry()
	for _, v := range []prometheus.Collector{
		noticeTypeCounter,
		applicationHandlerCallsCounter,
		applicationHandlerCallDurationHistogram,
		versionGague,
		subscriptionQueueLengthGauge,
		subscriptionQueueOldestMessageAgeGauge,
		relayDownloadersGauge,
		relayConnectionStateGauge,
		receivedEventsCounter,
		relayConnectionSubscriptionsGauge,
		relayRateLimitBackoffMsGauge,
		relayConnectionReceivedMessagesCounter,
		relayConnectionDisconnectionsCounter,
		storedRelayAddressesGauge,
		storedEventsGauge,
		eventsSentToRelayCounter,
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	} {
		if err := reg.Register(v); err != nil {
			return nil, errors.Wrap(err, "error registering a collector")
		}
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		var vcsRevision, vcsTime string
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" {
				vcsRevision = setting.Value
			}
			if setting.Key == "vcs.time" {
				vcsTime = setting.Value
			}
		}
		versionGague.With(prometheus.Labels{labelGo: buildInfo.GoVersion, labelVcsRevision: vcsRevision, labelVcsTime: vcsTime}).Set(1)
	}

	return &Prometheus{
		noticeTypeCounter:                       noticeTypeCounter,
		applicationHandlerCallsCounter:          applicationHandlerCallsCounter,
		applicationHandlerCallDurationHistogram: applicationHandlerCallDurationHistogram,
		relayDownloadersGauge:                   relayDownloadersGauge,
		subscriptionQueueLengthGauge:            subscriptionQueueLengthGauge,
		subscriptionQueueOldestMessageAgeGauge:  subscriptionQueueOldestMessageAgeGauge,
		relayConnectionStateGauge:               relayConnectionStateGauge,
		receivedEventsCounter:                   receivedEventsCounter,
		relayConnectionSubscriptionsGauge:       relayConnectionSubscriptionsGauge,
		relayRateLimitBackoffMsGauge:            relayRateLimitBackoffMsGauge,
		relayConnectionReceivedMessagesCounter:  relayConnectionReceivedMessagesCounter,
		relayConnectionDisconnectionsCounter:    relayConnectionDisconnectionsCounter,
		storedRelayAddressesGauge:               storedRelayAddressesGauge,
		storedEventsGauge:                       storedEventsGauge,
		eventsSentToRelayCounter:                eventsSentToRelayCounter,

		registry: reg,

		logger: logger.New("prometheus"),
	}, nil
}

func (p *Prometheus) StartApplicationCall(handlerName string) app.ApplicationCall {
	return NewApplicationCall(p, handlerName, p.logger)
}

func (p *Prometheus) ReportNumberOfRelayDownloaders(n int) {
	go func() {
		p.relayDownloadersGauge.Set(float64(n))
	}()
}

func (p *Prometheus) ReportRelayConnectionsState(m map[domain.RelayAddress]relays.RelayConnectionState) {
	go func() {
		p.relayConnectionStateGauge.Reset()
		for address, state := range m {
			p.relayConnectionStateGauge.With(
				prometheus.Labels{
					labelConnectionState: state.String(),
					labelAddress:         address.String(),
				},
			).Set(1)
		}
	}()
}

func (p *Prometheus) ReportReceivedEvent(address domain.RelayAddress) {
	go func() {
		p.receivedEventsCounter.With(prometheus.Labels{labelAddress: address.String()}).Inc()
	}()
}

func (p *Prometheus) ReportQueueLength(topic string, n int) {
	go func() {
		p.subscriptionQueueLengthGauge.With(prometheus.Labels{labelTopic: topic}).Set(float64(n))
	}()
}

func (p *Prometheus) ReportQueueOldestMessageAge(topic string, age time.Duration) {
	go func() {
		p.subscriptionQueueOldestMessageAgeGauge.With(prometheus.Labels{labelTopic: topic}).Set(age.Seconds())
	}()
}

func (p *Prometheus) ReportNumberOfSubscriptions(address domain.RelayAddress, n int) {
	go func() {
		p.relayConnectionSubscriptionsGauge.With(prometheus.Labels{
			labelAddress: address.String(),
		}).Set(float64(n))
	}()
}

func (p *Prometheus) ReportRateLimitBackoffMs(address domain.RelayAddress, n int) {
	go func() {
		p.relayRateLimitBackoffMsGauge.With(prometheus.Labels{
			labelAddress: address.HostWithoutPort(),
		}).Set(float64(n))
	}()
}

func (p *Prometheus) ReportMessageReceived(address domain.RelayAddress, messageType relays.MessageType, err *error) {
	labels := prometheus.Labels{
		labelAddress:     address.String(),
		labelMessageType: messageType.String(),
	}
	setResultLabel(labels, err)
	go func() {
		p.relayConnectionReceivedMessagesCounter.With(labels).Inc()
	}()
}

func (p *Prometheus) ReportRelayDisconnection(address domain.RelayAddress, err error) {
	go func() {
		p.relayConnectionDisconnectionsCounter.With(prometheus.Labels{
			labelAddress: address.String(),
			labelReason:  p.getDisconnectionReason(err),
		}).Inc()
	}()
}

func (p *Prometheus) ReportNumberOfStoredRelayAddresses(n int) {
	go func() {
		p.storedRelayAddressesGauge.Set(float64(n))
	}()
}

func (p *Prometheus) ReportNumberOfStoredEvents(n int) {
	go func() {
		p.storedEventsGauge.Set(float64(n))
	}()
}

func (p *Prometheus) ReportEventSentToRelay(address domain.RelayAddress, decision app.SendEventToRelayDecision, result app.SendEventToRelayResult) {
	go func() {
		p.eventsSentToRelayCounter.With(prometheus.Labels{
			labelAddress:  address.String(),
			labelDecision: decision.String(),
			labelResult:   result.String(),
		}).Inc()
	}()
}

func (p *Prometheus) ReportNotice(address domain.RelayAddress, noticeType relays.NoticeType) {
	go func() {
		p.noticeTypeCounter.With(prometheus.Labels{
			labelAddress:    address.String(),
			labelNoticeType: string(noticeType),
		}).Inc()
	}()
}

func (p *Prometheus) Registry() *prometheus.Registry {
	return p.registry
}

func (p *Prometheus) getDisconnectionReason(err error) string {
	if errors.Is(err, relays.DialError{}) {
		return "dial"
	}
	return "unknown"
}

type ApplicationCall struct {
	handlerName string
	p           *Prometheus
	start       time.Time
	logger      logging.Logger
}

func NewApplicationCall(p *Prometheus, handlerName string, logger logging.Logger) *ApplicationCall {
	return &ApplicationCall{
		p:           p,
		handlerName: handlerName,
		logger:      logger,
		start:       time.Now(),
	}
}

func (a *ApplicationCall) End(err *error) {
	duration := time.Since(a.start)

	l := a.logger.
		WithField("handlerName", a.handlerName).
		WithField("duration", duration)

	if err == nil {
		l.Error().Message("application call with an invalid error pointer")
	} else {
		if *err == nil {
			l.Trace().WithError(*err).Message("application call")
		} else {
			l.Debug().WithError(*err).Message("application call")
		}
	}

	labels := a.getLabels(err)
	a.p.applicationHandlerCallsCounter.With(labels).Inc()
	go func() {
		a.p.applicationHandlerCallDurationHistogram.With(labels).Observe(duration.Seconds())
	}()
}

func (a *ApplicationCall) getLabels(err *error) prometheus.Labels {
	labels := prometheus.Labels{
		labelHandlerName: a.handlerName,
	}

	setResultLabel(labels, err)

	return labels
}

func setResultLabel(labels prometheus.Labels, err *error) {
	if err == nil {
		labels[labelResult] = labelResultInvalidPointerPassed
	} else {
		if *err == nil {
			labels[labelResult] = labelResultSuccess
		} else {
			labels[labelResult] = labelResultError
		}
	}
}
