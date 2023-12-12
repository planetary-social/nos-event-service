package domain

import (
	"time"

	"github.com/boreq/errors"
	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/internal"
)

type Filter struct {
	filter nostr.Filter
}

func NewFilter(
	eventIDs []EventId,
	eventKinds []EventKind,
	eventTags []FilterTag,
	authors []PublicKey,
	since *time.Time,
	until *time.Time,
) (Filter, error) {
	filter := nostr.Filter{
		IDs:     nil,
		Kinds:   nil,
		Authors: nil,
		Tags:    nil,
		Since:   nil,
		Until:   nil,
		Limit:   0,
		Search:  "",
	}

	for _, eventID := range eventIDs {
		filter.IDs = append(filter.IDs, eventID.Hex())
	}

	for _, eventKind := range eventKinds {
		filter.Kinds = append(filter.Kinds, eventKind.Int())
	}

	if len(eventTags) > 0 {
		filter.Tags = make(nostr.TagMap)

		for _, eventTag := range eventTags {
			filter.Tags[eventTag.Name().String()] = []string{eventTag.Value()}
		}
	}

	for _, author := range authors {
		filter.Authors = append(filter.Authors, author.Hex())
	}

	if since != nil && until != nil {
		if !since.Before(*until) {
			return Filter{}, errors.New("since must be before until")
		}
	}

	if since != nil {
		filter.Since = internal.Pointer(nostr.Timestamp(since.Unix()))
	}

	if until != nil {
		filter.Until = internal.Pointer(nostr.Timestamp(until.Unix()))
	}

	return Filter{
		filter: filter,
	}, nil
}

func MustNewFilter(
	eventIDs []EventId,
	eventKinds []EventKind,
	eventTags []FilterTag,
	authors []PublicKey,
	since *time.Time,
	until *time.Time,
) Filter {
	v, err := NewFilter(eventIDs, eventKinds, eventTags, authors, since, until)
	if err != nil {
		panic(err)
	}
	return v
}

func (e Filter) Libfilter() nostr.Filter {
	return e.filter
}

func (e Filter) MarshalJSON() ([]byte, error) {
	return e.filter.MarshalJSON()
}

type FilterTag struct {
	name  EventTagName
	value string
}

func NewFilterTag(name EventTagName, value string) (FilterTag, error) {
	if value == "" {
		return FilterTag{}, errors.New("value can't be empty")
	}

	return FilterTag{name: name, value: value}, nil
}

func (f FilterTag) Name() EventTagName {
	return f.name
}

func (f FilterTag) Value() string {
	return f.value
}
