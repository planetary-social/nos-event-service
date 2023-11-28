package relays

import "github.com/planetary-social/nos-event-service/service/domain"

type EventOrEndOfSavedEvents struct {
	event domain.UnverifiedEvent
	eose  bool
}

func NewEventOrEndOfSavedEventsWithEvent(event domain.UnverifiedEvent) EventOrEndOfSavedEvents {
	return EventOrEndOfSavedEvents{event: event}
}

func NewEventOrEndOfSavedEventsWithEOSE() EventOrEndOfSavedEvents {
	return EventOrEndOfSavedEvents{eose: true}
}

func (e *EventOrEndOfSavedEvents) Event() domain.UnverifiedEvent {
	return e.event
}

func (e *EventOrEndOfSavedEvents) EOSE() bool {
	return e.eose
}
