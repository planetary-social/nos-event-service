package domain

import (
	"encoding/json"
	"time"

	"github.com/boreq/errors"
	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/internal"
)

type UnverifiedEvent struct {
	event event
}

func NewUnverifiedEvent(libevent nostr.Event) (UnverifiedEvent, error) {
	event, err := newEvent(libevent)
	if err != nil {
		return UnverifiedEvent{}, errors.Wrap(err, "error creating an event")
	}

	return UnverifiedEvent{
		event: event,
	}, nil
}

func (u UnverifiedEvent) Id() EventId {
	return u.event.id
}

func (u UnverifiedEvent) PubKey() PublicKey {
	return u.event.pubKey
}

func (u UnverifiedEvent) CreatedAt() time.Time {
	return u.event.createdAt
}

func (u UnverifiedEvent) Kind() EventKind {
	return u.event.kind
}

func (u UnverifiedEvent) Tags() []EventTag {
	return internal.CopySlice(u.event.tags)
}

func (u UnverifiedEvent) Raw() []byte {
	j, err := u.event.libevent.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return j
}

func (e UnverifiedEvent) String() string {
	return string(e.Raw())
}

type Event struct {
	event event
}

func NewEventFromRaw(raw []byte) (Event, error) {
	var libevent nostr.Event
	if err := json.Unmarshal(raw, &libevent); err != nil {
		return Event{}, errors.Wrap(err, "error unmarshaling")

	}
	return NewEvent(libevent)
}

func NewEvent(libevent nostr.Event) (Event, error) {
	unverifiedEvent, err := NewUnverifiedEvent(libevent)
	if err != nil {
		return Event{}, errors.Wrap(err, "error creating an unverified event")
	}

	return NewEventFromUnverifiedEvent(unverifiedEvent)
}

func NewEventFromUnverifiedEvent(event UnverifiedEvent) (Event, error) {
	ok, err := event.event.libevent.CheckSignature()
	if err != nil {
		return Event{}, errors.Wrap(err, "error checking signature")
	}

	if !ok {
		return Event{}, errors.New("invalid signature")
	}

	return Event(event), nil
}

func (e Event) Id() EventId {
	return e.event.id
}

func (e Event) PubKey() PublicKey {
	return e.event.pubKey
}

func (e Event) CreatedAt() time.Time {
	return e.event.createdAt
}

func (e Event) Kind() EventKind {
	return e.event.kind
}

func (e Event) Tags() []EventTag {
	return internal.CopySlice(e.event.tags)
}

func (e Event) HasInvalidProfileTags() bool {
	for _, tag := range e.event.tags {
		if !tag.IsProfile() {
			continue
		}

		if _, err := tag.Profile(); err != nil {
			return true
		}
	}
	return false
}

func (e Event) Content() string {
	return e.event.content
}

func (e Event) Libevent() nostr.Event {
	return e.event.libevent
}

func (e Event) MarshalJSON() ([]byte, error) {
	return e.event.libevent.MarshalJSON()
}

func (e Event) Raw() []byte {
	j, err := e.event.libevent.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return j
}

func (e Event) String() string {
	return string(e.Raw())
}

type event struct {
	id        EventId
	pubKey    PublicKey
	createdAt time.Time
	kind      EventKind
	tags      []EventTag
	content   string

	libevent nostr.Event
}

func newEvent(libevent nostr.Event) (event, error) {
	id, err := NewEventIdFromHex(libevent.ID)
	if err != nil {
		return event{}, errors.Wrap(err, "error creating an event id")
	}

	pubKey, err := NewPublicKeyFromHex(libevent.PubKey)
	if err != nil {
		return event{}, errors.Wrap(err, "error creating a pub key")
	}

	createdAt := time.Unix(int64(libevent.CreatedAt), 0).UTC()

	kind, err := NewEventKind(libevent.Kind)
	if err != nil {
		return event{}, errors.Wrap(err, "error creating event kind")
	}

	var tags []EventTag
	for _, libtag := range libevent.Tags {
		eventTag, err := NewEventTag(libtag)
		if err != nil {
			return event{}, errors.Wrap(err, "error creating a tag")
		}
		tags = append(tags, eventTag)
	}

	return event{
		id:        id,
		pubKey:    pubKey,
		createdAt: createdAt,
		kind:      kind,
		tags:      tags,
		content:   libevent.Content,

		libevent: libevent,
	}, nil
}
