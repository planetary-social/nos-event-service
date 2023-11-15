package domain

import (
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/internal"
)

type Filter struct {
	filter nostr.Filter
}

func NewFilter(
	eventIDs []EventId,
	eventKinds []EventKind,
	authors []PublicKey,
	since *time.Time,
) Filter {
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

	for _, author := range authors {
		filter.Authors = append(filter.Authors, author.Hex())
	}

	if since != nil {
		filter.Since = internal.Pointer(nostr.Timestamp(since.Unix()))
	}

	return Filter{
		filter: filter,
	}
}

func (e Filter) Libfilter() nostr.Filter {
	return e.filter
}

func (e Filter) MarshalJSON() ([]byte, error) {
	return e.filter.MarshalJSON()
}
