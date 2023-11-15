package domain

import (
	"encoding/json"

	"github.com/boreq/errors"
	"github.com/hashicorp/go-multierror"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

type MaybeRelayAddress struct {
	s string
}

func NewMaybeRelayAddress(s string) MaybeRelayAddress {
	return MaybeRelayAddress{s: s}
}

func (a MaybeRelayAddress) String() string {
	return a.s
}

// RelaysExtractor does its best to get relay addresses out of events. It should
// only error out on obvious programming errors but not on malformed user input.
type RelaysExtractor struct {
	logger logging.Logger
}

func NewRelaysExtractor(logger logging.Logger) *RelaysExtractor {
	return &RelaysExtractor{logger: logger.New("relayExtractor")}
}

func (e *RelaysExtractor) Extract(event Event) ([]MaybeRelayAddress, error) {
	switch event.Kind() {
	case EventKindMetadata:
		return e.extractFromMetadata(event)
	case EventKindContacts:
		return e.extractFromContacts(event)
	default:
		return nil, nil
	}
}

func (e *RelaysExtractor) extractFromMetadata(event Event) ([]MaybeRelayAddress, error) {
	if event.Kind() != EventKindMetadata {
		return nil, errors.New("invalid event kind")
	}

	results := internal.NewEmptySet[MaybeRelayAddress]()
	for _, tag := range event.Tags() {
		if tag.IsRelay() {
			maybeRelayAddress, err := tag.Relay()
			if err != nil {
				return nil, errors.Wrap(err, "error creating a maybe relay address")
			}
			results.Put(maybeRelayAddress)
		}
	}
	return results.List(), nil
}

func (e *RelaysExtractor) extractFromContacts(event Event) ([]MaybeRelayAddress, error) {
	if event.Kind() != EventKindContacts {
		return nil, errors.New("incorrect event kind")
	}

	if event.Content() == "" {
		return nil, nil
	}

	results := internal.NewEmptySet[MaybeRelayAddress]()

	var compoundErr error

	var asMap map[string]any
	if err := json.Unmarshal([]byte(event.Content()), &asMap); err == nil {
		for addressString := range asMap {
			results.Put(NewMaybeRelayAddress(addressString))
		}
		return results.List(), nil
	} else {
		compoundErr = multierror.Append(compoundErr, err)
	}

	var asSlice [][]any
	if err := json.Unmarshal([]byte(event.Content()), &asSlice); err == nil {
		for _, elements := range asSlice {
			if len(elements) > 0 {
				s, ok := elements[0].(string)
				if ok {
					results.Put(NewMaybeRelayAddress(s))
				}
			}
		}
		return results.List(), nil
	} else {
		compoundErr = multierror.Append(compoundErr, err)
	}

	e.logger.
		Debug().
		WithError(compoundErr).
		WithField("content", event.Content()).
		Message("can't unmarshal")

	return nil, nil
}
