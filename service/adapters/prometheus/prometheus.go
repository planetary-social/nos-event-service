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
)

type Prometheus struct {
	applicationHandlerCallsCounter          *prometheus.CounterVec
	applicationHandlerCallDurationHistogram *prometheus.HistogramVec
	relayDownloadersGauge                   prometheus.Gauge
	subscriptionQueueLengthGauge            *prometheus.GaugeVec
	relayConnectionStateGauge               *prometheus.GaugeVec
	receivedEventsCounter                   *prometheus.CounterVec
	relayConnectionSubscriptionsGauge       *prometheus.GaugeVec
	relayConnectionReceivedMessagesCounter  *prometheus.CounterVec

	registry *prometheus.Registry

	logger logging.Logger
}

func NewPrometheus(logger logging.Logger) (*Prometheus, error) {
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
	relayDownloadersGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "relay_downloader_gauge",
			Help: "Number of running relay downloaders.",
		},
	)
	subscriptionQueueLengthGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "subscription_queue_length",
			Help: "Number of events in the subscription queue.",
		},
		[]string{labelTopic},
	)
	versionGague := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "version",
			Help: "This metric exists just to put a commit label on it.",
		},
		[]string{labelVcsRevision, labelVcsTime, labelGo},
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
	relayConnectionReceivedMessagesCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relay_connection_received_messages_counter",
			Help: "Number of received messages and their processing result.",
		},
		[]string{labelAddress, labelMessageType, labelResult},
	)

	reg := prometheus.NewRegistry()
	for _, v := range []prometheus.Collector{
		applicationHandlerCallsCounter,
		applicationHandlerCallDurationHistogram,
		relayDownloadersGauge,
		subscriptionQueueLengthGauge,
		versionGague,
		relayConnectionStateGauge,
		receivedEventsCounter,
		relayConnectionSubscriptionsGauge,
		relayConnectionReceivedMessagesCounter,
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
		applicationHandlerCallsCounter:          applicationHandlerCallsCounter,
		applicationHandlerCallDurationHistogram: applicationHandlerCallDurationHistogram,
		relayDownloadersGauge:                   relayDownloadersGauge,
		subscriptionQueueLengthGauge:            subscriptionQueueLengthGauge,
		relayConnectionStateGauge:               relayConnectionStateGauge,
		receivedEventsCounter:                   receivedEventsCounter,
		relayConnectionSubscriptionsGauge:       relayConnectionSubscriptionsGauge,
		relayConnectionReceivedMessagesCounter:  relayConnectionReceivedMessagesCounter,

		registry: reg,

		logger: logger.New("prometheus"),
	}, nil
}

func (p *Prometheus) StartApplicationCall(handlerName string) app.ApplicationCall {
	return NewApplicationCall(p, handlerName, p.logger)
}

func (p *Prometheus) ReportNumberOfRelayDownloaders(n int) {
	p.relayDownloadersGauge.Set(float64(n))
}

func (p *Prometheus) ReportRelayConnectionsState(m map[domain.RelayAddress]relays.RelayConnectionState) {
	p.relayConnectionStateGauge.Reset()
	for address, state := range m {
		p.relayConnectionStateGauge.With(
			prometheus.Labels{
				labelConnectionState: state.String(),
				labelAddress:         address.String(),
			},
		).Set(1)
	}
}

func (p *Prometheus) ReportReceivedEvent(address domain.RelayAddress) {
	p.receivedEventsCounter.With(prometheus.Labels{labelAddress: address.String()}).Inc()
}

func (p *Prometheus) ReportQueueLength(topic string, n int) {
	p.subscriptionQueueLengthGauge.With(prometheus.Labels{labelTopic: topic}).Set(float64(n))
}

func (p *Prometheus) ReportNumberOfSubscriptions(address domain.RelayAddress, n int) {
	p.relayConnectionSubscriptionsGauge.With(prometheus.Labels{
		labelAddress: address.String(),
	}).Set(float64(n))
}

func (p *Prometheus) ReportMessageReceived(address domain.RelayAddress, messageType relays.MessageType, err *error) {
	labels := prometheus.Labels{
		labelAddress:     address.String(),
		labelMessageType: messageType.String(),
	}
	setResultLabel(labels, err)
	p.relayConnectionReceivedMessagesCounter.With(labels).Inc()
}

func (p *Prometheus) Registry() *prometheus.Registry {
	return p.registry
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
		l.Debug().WithError(*err).Message("application call")
	}

	labels := a.getLabels(err)
	a.p.applicationHandlerCallsCounter.With(labels).Inc()
	a.p.applicationHandlerCallDurationHistogram.With(labels).Observe(duration.Seconds())
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
