package transport

import (
	"github.com/boreq/errors"
	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type MessageReq struct {
	subscriptionID SubscriptionID
	filters        []domain.Filter
}

func NewMessageReq(subscriptionID SubscriptionID, filters []domain.Filter) MessageReq {
	return MessageReq{subscriptionID: subscriptionID, filters: filters}
}

func (m MessageReq) MarshalJSON() ([]byte, error) {
	env := nostr.ReqEnvelope{
		SubscriptionID: m.subscriptionID.String(),
		Filters:        nostr.Filters{},
	}
	for _, filter := range m.filters {
		env.Filters = append(env.Filters, filter.Libfilter())
	}
	return env.MarshalJSON()
}

type MessageClose struct {
	subscriptionID SubscriptionID
}

func NewMessageClose(subscriptionID SubscriptionID) MessageClose {
	return MessageClose{subscriptionID: subscriptionID}
}

func (m MessageClose) MarshalJSON() ([]byte, error) {
	env := nostr.CloseEnvelope(m.subscriptionID.String())
	return env.MarshalJSON()
}

type MessageEvent struct {
	event domain.Event
}

func NewMessageEvent(event domain.Event) MessageEvent {
	return MessageEvent{event: event}
}

func (m MessageEvent) MarshalJSON() ([]byte, error) {
	env := nostr.EventEnvelope{
		SubscriptionID: nil,
		Event:          m.event.Libevent(),
	}
	return env.MarshalJSON()
}

type MessageOK struct {
	eventID string
	err     error
}

func NewMessageOKWithSuccess(eventId string) MessageOK {
	return MessageOK{
		eventID: eventId,
		err:     nil,
	}
}

func NewMessageOKWithError(eventId string, message string) MessageOK {
	return MessageOK{
		eventID: eventId,
		err:     errors.New(message),
	}
}

func (m MessageOK) MarshalJSON() ([]byte, error) {
	env := nostr.OKEnvelope{
		EventID: m.eventID,
	}
	if m.err == nil {
		env.OK = true
		env.Reason = ""
	} else {
		env.OK = false
		env.Reason = m.err.Error()
	}
	return env.MarshalJSON()
}
