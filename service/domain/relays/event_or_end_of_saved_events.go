package relays

import "github.com/planetary-social/nos-event-service/service/domain"

type EventOrEndOfSavedEvents struct {
	event domain.Event
	eose  bool
}

func NewEventOrEndOfSavedEventsWithEvent(event domain.Event) EventOrEndOfSavedEvents {
	return EventOrEndOfSavedEvents{event: event}
}

func NewEventOrEndOfSavedEventsWithEOSE() EventOrEndOfSavedEvents {
	return EventOrEndOfSavedEvents{eose: true}
}

func (e *EventOrEndOfSavedEvents) Event() domain.Event {
	return e.event
}

func (e *EventOrEndOfSavedEvents) EOSE() bool {
	return e.eose
}
