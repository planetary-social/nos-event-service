package domain_test

import (
	"strings"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestEvent_HasInvalidProfileTags(t *testing.T) {
	testCases := []struct {
		Name   string
		Event  domain.Event
		Result bool
	}{
		{
			Name:   "no_tags",
			Event:  fixtures.Event(fixtures.SomeEventKind(), nil, fixtures.SomeString()),
			Result: false,
		},
		{
			Name: "malformed_tag",
			Event: fixtures.Event(
				fixtures.SomeEventKind(),
				[]domain.EventTag{
					domain.MustNewEventTag([]string{
						"p", "something",
					}),
				},
				fixtures.SomeString(),
			),
			Result: true,
		},
		{
			Name: "correct_tag",
			Event: fixtures.Event(
				fixtures.SomeEventKind(),
				[]domain.EventTag{
					domain.MustNewEventTag([]string{
						"p", fixtures.SomePublicKey().Hex(),
					}),
				},
				fixtures.SomeString(),
			),
			Result: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			require.Equal(t, testCase.Result, testCase.Event.HasInvalidProfileTags())
		})
	}
}
func TestEvent_HasInvalidRTags(t *testing.T) {
	largeString := strings.Repeat("a", 2000)

	testCases := []struct {
		Name   string
		Event  domain.Event
		Result bool
	}{
		{
			Name:   "valid_single_relay",
			Event:  fixtures.Event(fixtures.SomeEventKind(), []domain.EventTag{domain.MustNewEventTag([]string{"r", "wss://example-relay.com"})}, fixtures.SomeString()),
			Result: false,
		},
		{
			Name: "valid_multiple_relays",
			Event: fixtures.Event(
				fixtures.SomeEventKind(),
				[]domain.EventTag{
					domain.MustNewEventTag([]string{"r", "wss://relay1.com"}),
					domain.MustNewEventTag([]string{"r", "wss://relay2.com", "read"}),
					domain.MustNewEventTag([]string{"r", "wss://relay3.com", "write"}),
				},
				fixtures.SomeString(),
			),
			Result: false,
		},
		{
			Name: "invalid_concatenated_relays",
			Event: fixtures.Event(
				fixtures.SomeEventKind(),
				[]domain.EventTag{domain.MustNewEventTag([]string{"r", "wss://foobar.com" + largeString})},
				fixtures.SomeString(),
			),
			Result: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			require.Equal(t, testCase.Result, testCase.Event.HasInvalidRTags())
		})
	}
}
