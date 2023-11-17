package domain

import (
	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

var ErrNotContactsEvent = errors.New("this is not a contacts event")

// ContactsExtractor does its best to get relay addresses out of events. It
// should only error out on obvious programming errors but not on malformed user
// input.
type ContactsExtractor struct {
	logger logging.Logger
}

func NewContactsExtractor(logger logging.Logger) *ContactsExtractor {
	return &ContactsExtractor{logger: logger.New("contactsExtractor")}
}

// Extract returns ErrNotContactsEvent if the event isn't an event that
// normally contains contacts. This is to distinguish between events that
// contain zero contacts and events that don't ever contain contacts.
func (e *ContactsExtractor) Extract(event Event) ([]PublicKey, error) {
	switch event.Kind() {
	case EventKindContacts:
		return e.extractFromContacts(event)
	default:
		return nil, ErrNotContactsEvent
	}
}

func (e *ContactsExtractor) extractFromContacts(event Event) ([]PublicKey, error) {
	if event.Kind() != EventKindContacts {
		return nil, errors.New("invalid event kind")
	}

	results := internal.NewEmptySet[PublicKey]()
	for _, tag := range event.Tags() {
		if tag.IsProfile() {
			publicKey, err := tag.Profile()
			if err != nil {
				e.logger.
					Debug().
					WithError(err).
					WithField("name", tag.Name()).
					WithField("value", tag.FirstValue()).
					Message("error grabbing the profile tag")
				continue
			}
			results.Put(publicKey)
		}
	}
	return results.List(), nil
}

func ShouldReplaceContactsEvent(old, new Event) (bool, error) {
	if old.Kind() != EventKindContacts || new.Kind() != EventKindContacts {
		return false, errors.New("invalid event kind")
	}
	return new.CreatedAt().After(old.CreatedAt()), nil
}
