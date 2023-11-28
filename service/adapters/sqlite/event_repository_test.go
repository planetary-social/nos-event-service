package sqlite_test

import (
	"context"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/stretchr/testify/require"
)

func TestEventRepository_GetReturnsPredefinedError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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

		err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
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
