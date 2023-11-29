package app_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/cmd/event-service/di"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestGetEventsHandler(t *testing.T) {
	someEventID := fixtures.SomeEventID()

	var oneHundredEvents []domain.Event
	for i := 0; i < 100; i++ {
		oneHundredEvents = append(oneHundredEvents, fixtures.SomeEvent())
	}
	event := fixtures.SomeEvent()

	testCases := []struct {
		Name                           string
		After                          *domain.EventId
		EventRepositoryListReturnValue []domain.Event
		ExpectedEventRepositoryCalls   []mocks.EventRepositoryListCall
		ExpectedResult                 app.GetEventsResult
	}{
		{
			Name:  "one_event",
			After: internal.Pointer(someEventID),
			EventRepositoryListReturnValue: []domain.Event{
				event,
			},
			ExpectedEventRepositoryCalls: []mocks.EventRepositoryListCall{
				{
					After: internal.Pointer(someEventID),
					Limit: 101,
				},
			},
			ExpectedResult: app.NewGetEventsResult(
				[]domain.Event{
					event,
				},
				false,
			),
		},
		{
			Name:                           "one_hundred_events",
			After:                          internal.Pointer(someEventID),
			EventRepositoryListReturnValue: oneHundredEvents,
			ExpectedEventRepositoryCalls: []mocks.EventRepositoryListCall{
				{
					After: internal.Pointer(someEventID),
					Limit: 101,
				},
			},
			ExpectedResult: app.NewGetEventsResult(
				oneHundredEvents,
				false,
			),
		},
		{
			Name:                           "one_hundred_events_plus_one",
			After:                          internal.Pointer(someEventID),
			EventRepositoryListReturnValue: append(oneHundredEvents, event),
			ExpectedEventRepositoryCalls: []mocks.EventRepositoryListCall{
				{
					After: internal.Pointer(someEventID),
					Limit: 101,
				},
			},
			ExpectedResult: app.NewGetEventsResult(
				oneHundredEvents,
				true,
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			ctx := fixtures.TestContext(t)
			ts, err := di.BuildTestApplication(ctx, t)
			require.NoError(t, err)

			ts.EventRepository.ListReturnValue = testCase.EventRepositoryListReturnValue

			result, err := ts.GetEventsHandler.Handle(ctx, app.NewGetEvents(testCase.After))
			require.NoError(t, err)

			fixtures.RequireEqualEventSlices(t, testCase.ExpectedResult.Events(), result.Events())
			require.Equal(t, testCase.ExpectedResult.ThereIsMoreEvents(), result.ThereIsMoreEvents())

			require.Equal(t, testCase.ExpectedEventRepositoryCalls, ts.EventRepository.ListCalls)
		})
	}
}
