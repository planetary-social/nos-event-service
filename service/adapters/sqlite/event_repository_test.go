package sqlite_test

import (
	"context"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/stretchr/testify/require"
)

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
