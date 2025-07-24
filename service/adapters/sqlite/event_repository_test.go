package sqlite_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestEventRepository_GetReturnsPredefinedError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		_, err := adapters.EventRepository.Get(ctx, fixtures.SomeEventID())
		require.ErrorIs(t, err, app.ErrEventNotFound)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_SavingTheSameEventTwiceReturnsNoErrors(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event := fixtures.SomeEvent()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_ItIsPossibleToSaveAndGetEvents(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event := fixtures.SomeEvent()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		readEvent, err := adapters.EventRepository.Get(ctx, event.Id())
		require.NoError(t, err)
		require.Equal(t, event.Raw(), readEvent.Raw())

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_CountCountsSavedEvents(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		n, err := adapters.EventRepository.Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		return nil
	})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
			err := adapters.EventRepository.Save(ctx, fixtures.SomeEvent())
			require.NoError(t, err)

			return nil
		})
		require.NoError(t, err)

		err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
			n, err := adapters.EventRepository.Count(ctx)
			require.NoError(t, err)
			require.Equal(t, i+1, n)

			return nil
		})
		require.NoError(t, err)
	}
}

func TestEventRepository_ExistsChecksIfEventsExist(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event1 := fixtures.SomeEvent()
	event2 := fixtures.SomeEvent()

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		ok, err := adapters.EventRepository.Exists(ctx, event1.Id())
		require.NoError(t, err)
		require.False(t, ok)

		ok, err = adapters.EventRepository.Exists(ctx, event2.Id())
		require.NoError(t, err)
		require.False(t, ok)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		ok, err := adapters.EventRepository.Exists(ctx, event1.Id())
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = adapters.EventRepository.Exists(ctx, event2.Id())
		require.NoError(t, err)
		require.False(t, ok)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_ListReturnsNoEventsIfRepositoryIsEmpty(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		events, err := adapters.EventRepository.List(ctx, nil, 10)
		require.NoError(t, err)
		require.Empty(t, events)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_ListReturnsEventsIfRepositoryIsNotEmpty(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	var savedEvents []domain.Event
	for i := 0; i < 10; i++ {
		savedEvents = append(savedEvents, fixtures.SomeEvent())
	}

	slices.SortFunc(savedEvents, func(a, b domain.Event) int {
		return strings.Compare(a.Id().Hex(), b.Id().Hex())
	})

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		for _, event := range savedEvents {
			err := adapters.EventRepository.Save(ctx, event)
			require.NoError(t, err)
		}
		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		events, err := adapters.EventRepository.List(ctx, nil, 2)
		require.NoError(t, err)
		fixtures.RequireEqualEventSlices(t,
			[]domain.Event{
				savedEvents[0],
				savedEvents[1],
			},
			events,
		)

		events, err = adapters.EventRepository.List(ctx, internal.Pointer(savedEvents[1].Id()), 2)
		require.NoError(t, err)
		fixtures.RequireEqualEventSlices(t,
			[]domain.Event{
				savedEvents[2],
				savedEvents[3],
			},
			events,
		)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_Delete(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event := fixtures.SomeEvent()

	// Save and verify existence
	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		exists, err := adapters.EventRepository.Exists(ctx, event.Id())
		require.NoError(t, err)
		require.True(t, exists)

		return nil
	})
	require.NoError(t, err)

	// Delete and verify deletion
	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Delete(ctx, event.Id())
		require.NoError(t, err)

		exists, err := adapters.EventRepository.Exists(ctx, event.Id())
		require.NoError(t, err)
		require.False(t, exists)

		// Delete again should not error
		err = adapters.EventRepository.Delete(ctx, event.Id())
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_MarkAsProcessed(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event := fixtures.SomeEvent()

	// Save an event
	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Mark it as processed
	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.MarkAsProcessed(ctx, event.Id())
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Verify it still exists
	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		exists, err := adapters.EventRepository.Exists(ctx, event.Id())
		require.NoError(t, err)
		require.True(t, exists)
		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_DeleteProcessedEventsBefore(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	// Create and save multiple events
	event1 := fixtures.SomeEvent()
	event2 := fixtures.SomeEvent()
	event3 := fixtures.SomeEvent()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)
		err = adapters.EventRepository.Save(ctx, event2)
		require.NoError(t, err)
		err = adapters.EventRepository.Save(ctx, event3)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Mark event1 and event2 as processed
	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.MarkAsProcessed(ctx, event1.Id())
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Small delay to ensure different timestamps

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.MarkAsProcessed(ctx, event2.Id())
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Delete processed events before "now" (should delete both event1 and event2)
	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		deletedCount, err := adapters.EventRepository.DeleteProcessedEventsBefore(ctx, time.Now().Add(1*time.Second))
		require.NoError(t, err)
		require.Equal(t, 2, deletedCount)
		return nil
	})
	require.NoError(t, err)

	// Verify event1 and event2 are deleted
	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		exists, err := adapters.EventRepository.Exists(ctx, event1.Id())
		require.NoError(t, err)
		require.False(t, exists)

		exists, err = adapters.EventRepository.Exists(ctx, event2.Id())
		require.NoError(t, err)
		require.False(t, exists)

		// Verify event3 still exists (was not processed)
		exists, err = adapters.EventRepository.Exists(ctx, event3.Id())
		require.NoError(t, err)
		require.True(t, exists)

		return nil
	})
	require.NoError(t, err)
}
