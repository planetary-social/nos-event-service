package app_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/stretchr/testify/require"
)

func TestProcessSavedEventHandler_MarksEventAsProcessedAfterProcessing(t *testing.T) {
	ctx := fixtures.TestContext(t)

	// Create mocks
	eventRepo := mocks.NewEventRepository()
	relayRepo := mocks.NewRelayRepository()
	contactRepo := mocks.NewContactRepository()
	metrics := mocks.NewMetrics()
	logger := logging.NewNopLogger()

	// Create mock transaction provider with all repositories
	transactionProvider := fixtures.NewTransactionProvider(eventRepo, relayRepo, contactRepo)

	// Create the handler with mocks
	handler := app.NewProcessSavedEventHandler(
		transactionProvider,
		mocks.NewRelaysExtractor(),
		mocks.NewContactsExtractor(),
		mocks.NewExternalEventPublisher(),
		mocks.NewEventSender(),
		logger,
		metrics,
	)

	event := fixtures.SomeEvent()

	// Save the event
	err := eventRepo.Save(ctx, event)
	require.NoError(t, err)

	// Process the event
	err = handler.Handle(ctx, app.NewProcessSavedEvent(event.Id()))
	require.NoError(t, err)

	// Verify the event was marked as processed
	require.Len(t, eventRepo.MarkAsProcessedCalls, 1, "event should be marked as processed")
	require.Equal(t, event.Id(), eventRepo.MarkAsProcessedCalls[0].EventID)

	// Verify the event still exists (not deleted)
	exists, err := eventRepo.Exists(ctx, event.Id())
	require.NoError(t, err)
	require.True(t, exists, "event should still exist after being marked as processed")
}
