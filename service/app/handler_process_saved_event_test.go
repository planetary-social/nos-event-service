package app_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/stretchr/testify/require"
)

func TestProcessSavedEventHandler_DeletesEventAfterProcessing(t *testing.T) {
	ctx := fixtures.TestContext(t)

	// Create mocks
	eventRepo := mocks.NewEventRepository()
	metrics := mocks.NewMetrics()
	logger := logging.NewNopLogger()

	// Create the handler with mocks
	handler := app.NewProcessSavedEventHandler(
		fixtures.NewTransactionProvider(eventRepo),
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

	// Verify the event was deleted
	exists, err := eventRepo.Exists(ctx, event.Id())
	require.NoError(t, err)
	require.False(t, exists, "event should be deleted after processing")
}
