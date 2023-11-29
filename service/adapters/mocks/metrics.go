package mocks

import (
	"time"

	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type Metrics struct {
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m Metrics) StartApplicationCall(handlerName string) app.ApplicationCall {
	return NewApplicationCall()
}

func (m Metrics) ReportNumberOfRelayDownloaders(n int) {
}

func (m Metrics) ReportReceivedEvent(address domain.RelayAddress) {
}

func (m Metrics) ReportQueueLength(topic string, n int) {
}

func (m Metrics) ReportQueueOldestMessageAge(topic string, age time.Duration) {
}

func (m Metrics) ReportNumberOfStoredRelayAddresses(n int) {
}

func (m Metrics) ReportNumberOfStoredEvents(n int) {
}

func (m Metrics) ReportEventSentToRelay(address domain.RelayAddress, decision app.SendEventToRelayDecision, result app.SendEventToRelayResult) {
}

type ApplicationCall struct {
}

func NewApplicationCall() *ApplicationCall {
	return &ApplicationCall{}
}

func (a ApplicationCall) End(err *error) {
}
