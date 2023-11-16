package domain_test

import (
	"sort"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestContactsExtractor(t *testing.T) {
	publicKey1 := fixtures.SomePublicKey()
	publicKey2 := fixtures.SomePublicKey()

	testCases := []struct {
		Name string

		Kind domain.EventKind
		Tags []domain.EventTag

		Result        []domain.PublicKey
		ExpectedErrIs error
	}{
		{
			Name: "contacts",

			Kind: domain.EventKindContacts,
			Tags: []domain.EventTag{
				domain.MustNewEventTag([]string{
					"p", publicKey1.Hex(),
				}),
				domain.MustNewEventTag([]string{
					"p", publicKey2.Hex(),
				}),
			},

			Result: []domain.PublicKey{
				publicKey1,
				publicKey2,
			},
			ExpectedErrIs: nil,
		},
		{
			Name: "not_contacts",

			Kind: domain.EventKindNote,
			Tags: nil,

			Result:        nil,
			ExpectedErrIs: domain.ErrNotContactsEvent,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			event := fixtures.Event(testCase.Kind, testCase.Tags, fixtures.SomeString())

			logger := fixtures.TestLogger(t)
			extractor := domain.NewContactsExtractor(logger)

			result, err := extractor.Extract(event)
			if testCase.ExpectedErrIs != nil {
				require.ErrorIs(t, err, testCase.ExpectedErrIs)
			} else {
				require.NoError(t, err)

				sort.Slice(result, func(i, j int) bool {
					return result[i].Hex() < result[j].Hex()
				})

				sort.Slice(testCase.Result, func(i, j int) bool {
					return testCase.Result[i].Hex() < testCase.Result[j].Hex()
				})

				require.Equal(t, testCase.Result, result)
			}
		})
	}
}
