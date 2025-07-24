package app_test

import (
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/stretchr/testify/require"
)

func TestCleanupProcessedEventsHandler_DeletesOldProcessedEvents(t *testing.T) {
	ctx := fixtures.TestContext(t)

	// Create mock repositories
	eventRepo := mocks.NewEventRepository()
	adapters := app.Adapters{
		Events: eventRepo,
	}

	// Create transaction provider
	transactionProvider := mocks.NewTransactionProvider(adapters)

	// Create handler
	handler := app.NewCleanupProcessedEventsHandler(
		transactionProvider,
		fixtures.TestLogger(t),
		mocks.NewMetrics(),
	)

	// Simulate some processed events with different timestamps
	now := time.Now()
	oldEvent := fixtures.SomeEvent()
	recentEvent := fixtures.SomeEvent()

	// Mark events as processed at different times
	eventRepo.ProcessedEvents[oldEvent.Id().Hex()] = now.Add(-2 * time.Hour)       // Old event
	eventRepo.ProcessedEvents[recentEvent.Id().Hex()] = now.Add(-30 * time.Minute) // Recent event

	// Save the events
	eventRepo.SaveCalls = []mocks.EventRepositorySaveCall{
		{Event: oldEvent},
		{Event: recentEvent},
	}

	// Run cleanup
	err := handler.Handle(ctx, app.NewCleanupProcessedEvents())
	require.NoError(t, err)

	// Check that only the old event was deleted
	require.Len(t, eventRepo.DeleteProcessedEventsBeforeCalls, 1)

	// The old event should be gone from ProcessedEvents
	_, oldExists := eventRepo.ProcessedEvents[oldEvent.Id().Hex()]
	require.False(t, oldExists, "old event should have been deleted")

	// The recent event should still exist
	_, recentExists := eventRepo.ProcessedEvents[recentEvent.Id().Hex()]
	require.True(t, recentExists, "recent event should still exist")
}

func TestCleanupProcessedEventsHandler_HandlesNoEventsToDelete(t *testing.T) {
	ctx := fixtures.TestContext(t)

	// Create mock repositories
	eventRepo := mocks.NewEventRepository()
	adapters := app.Adapters{
		Events: eventRepo,
	}

	// Create transaction provider
	transactionProvider := mocks.NewTransactionProvider(adapters)

	// Create handler
	handler := app.NewCleanupProcessedEventsHandler(
		transactionProvider,
		fixtures.TestLogger(t),
		mocks.NewMetrics(),
	)

	// Add only recent events
	now := time.Now()
	recentEvent := fixtures.SomeEvent()
	eventRepo.ProcessedEvents[recentEvent.Id().Hex()] = now.Add(-5 * time.Minute)

	// Run cleanup
	err := handler.Handle(ctx, app.NewCleanupProcessedEvents())
	require.NoError(t, err)

	// Check that delete was called but nothing was deleted
	require.Len(t, eventRepo.DeleteProcessedEventsBeforeCalls, 1)
	require.Len(t, eventRepo.ProcessedEvents, 1, "recent event should still exist")
}
