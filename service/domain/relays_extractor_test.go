package domain_test

import (
	"sort"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestRelaysExtractor(t *testing.T) {
	testCases := []struct {
		Name string

		Kind    domain.EventKind
		Tags    domain.EventTag
		Content string

		Result []domain.MaybeRelayAddress
	}{
		{
			Name: "contacts_with_map",
			Kind: domain.EventKindContacts,
			//Tags: domain.NewEventTag([]string{
			//	"p, "
			//}),
			Content: `{"wss://relay.damus.io":{"read":true,"write":true},"wss://nostr.bitcoiner.social":{"read":true,"write":true}}`,

			Result: []domain.MaybeRelayAddress{
				domain.NewMaybeRelayAddress("wss://relay.damus.io"),
				domain.NewMaybeRelayAddress("wss://nostr.bitcoiner.social"),
			},
		},
		{
			Name: "contacts_with_slice",
			Kind: domain.EventKindContacts,
			//Tags: domain.NewEventTag([]string{
			//	"p, "
			//}),
			Content: `[["wss://nostr.wine",{"write":true,"read":true}],["wss://relay.current.fyi",{"read":true,"write":true}]]`,

			Result: []domain.MaybeRelayAddress{
				domain.NewMaybeRelayAddress("wss://nostr.wine"),
				domain.NewMaybeRelayAddress("wss://relay.current.fyi"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			event := fixtures.Event(testCase.Kind, nil, testCase.Content)

			logger := fixtures.TestLogger(t)
			extractor := domain.NewRelaysExtractor(logger)

			result, err := extractor.Extract(event)
			require.NoError(t, err)

			sort.Slice(result, func(i, j int) bool {
				return result[i].String() < result[j].String()
			})

			sort.Slice(testCase.Result, func(i, j int) bool {
				return testCase.Result[i].String() < testCase.Result[j].String()
			})

			require.Equal(t, testCase.Result, result)
		})
	}
}
