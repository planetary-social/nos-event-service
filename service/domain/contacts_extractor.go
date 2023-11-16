package domain

import (
	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

// ContactsExtractor does its best to get relay addresses out of events. It
// should only error out on obvious programming errors but not on malformed user
// input.
type ContactsExtractor struct {
	logger logging.Logger
}

func NewContactsExtractor(logger logging.Logger) *ContactsExtractor {
	return &ContactsExtractor{logger: logger.New("contactsExtractor")}
}

func (e *ContactsExtractor) Extract(event Event) ([]PublicKey, error) {
	switch event.Kind() {
	case EventKindContacts:
		return e.extractFromContacts(event)
	default:
		return nil, nil
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
				return nil, errors.Wrap(err, "error grabbing the profile tag")
			}
			results.Put(publicKey)
		}
	}
	return results.List(), nil
}
