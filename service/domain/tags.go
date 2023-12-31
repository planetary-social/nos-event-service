package domain

import (
	"unicode"

	"github.com/boreq/errors"
)

var (
	TagProfile = MustNewEventTagName("p")
	TagRelay   = MustNewEventTagName("r")
)

type EventTag struct {
	name EventTagName
	tag  []string
}

func NewEventTag(tag []string) (EventTag, error) {
	if len(tag) < 2 {
		return EventTag{}, errors.New("tag needs at least two fields I recon")
	}

	name, err := NewEventTagName(tag[0])
	if err != nil {
		return EventTag{}, errors.Wrap(err, "invalid tag name")
	}

	return EventTag{name: name, tag: tag}, nil
}

func MustNewEventTag(tag []string) EventTag {
	v, err := NewEventTag(tag)
	if err != nil {
		panic(err)
	}
	return v
}

func (e EventTag) Name() EventTagName {
	return e.name
}

func (e EventTag) FirstValue() string {
	return e.tag[1]
}

func (e EventTag) FirstValueIsAnEmptyString() bool {
	return e.FirstValue() == ""
}

func (e EventTag) IsProfile() bool {
	return e.name == TagProfile
}

func (e EventTag) IsRelay() bool {
	return e.name == TagRelay
}

func (e EventTag) Profile() (PublicKey, error) {
	if !e.IsProfile() {
		return PublicKey{}, errors.New("not a profile tag")
	}
	return NewPublicKeyFromHex(e.tag[1])
}

func (e EventTag) Relay() (MaybeRelayAddress, error) {
	if !e.IsRelay() {
		return MaybeRelayAddress{}, errors.New("not a relay address tag")
	}
	return NewMaybeRelayAddress(e.tag[1]), nil
}

type EventTagName struct {
	s string
}

func NewEventTagName(s string) (EventTagName, error) {
	if s == "" {
		return EventTagName{}, errors.New("missing tag name")
	}

	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' {
			return EventTagName{}, errors.New("tag name should only contain letters and numbers")
		}
	}

	return EventTagName{s}, nil
}

func MustNewEventTagName(s string) EventTagName {
	v, err := NewEventTagName(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (e EventTagName) String() string {
	return e.s
}
