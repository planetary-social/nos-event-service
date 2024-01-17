package domain

import (
	"github.com/boreq/errors"
)

var (
	EventKindMetadata               = MustNewEventKind(0)
	EventKindNote                   = MustNewEventKind(1)
	EventKindRecommendedRelay       = MustNewEventKind(2)
	EventKindContacts               = MustNewEventKind(3)
	EventKindEncryptedDirectMessage = MustNewEventKind(4)
	EventKindReaction               = MustNewEventKind(7)
	EventKindRelayListMetadata      = MustNewEventKind(10002)
	// TODO: This should be changed to 30078
	// See https://github.com/nostr-protocol/nips/blob/master/78.md , 6666 is
	// reserved by nip 90
	EventKindRegistration = MustNewEventKind(6666)
)

type EventKind struct {
	k int
}

func NewEventKind(k int) (EventKind, error) {
	if k < 0 {
		return EventKind{}, errors.New("kind must be positive")
	}
	return EventKind{k}, nil
}

func MustNewEventKind(k int) EventKind {
	v, err := NewEventKind(k)
	if err != nil {
		panic(err)
	}
	return v
}

func (k EventKind) Int() int {
	return k.k
}
