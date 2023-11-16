package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/stretchr/testify/require"
)

func TestRelayRepository_ListingWithoutSavingReturnsNoResults(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		addresses, err := adapters.RelayRepository.List(ctx)
		require.NoError(t, err)
		require.Empty(t, addresses)

		return nil
	})
	require.NoError(t, err)
}

func TestRelayRepository_ToSaveRelaysEventMustExist(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		return adapters.RelayRepository.Save(ctx, fixtures.SomeEventID(), fixtures.SomeMaybeRelayAddress())
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRelayRepository_ItIsPossibleToListSavedData(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event1 := fixtures.SomeEvent()
	address1 := fixtures.SomeMaybeRelayAddress()

	event2 := fixtures.SomeEvent()
	address2 := fixtures.SomeMaybeRelayAddress()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)

		err = adapters.EventRepository.Save(ctx, event2)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.RelayRepository.Save(ctx, event1.Id(), address1)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		addresses, err := adapters.RelayRepository.List(ctx)
		require.NoError(t, err)
		require.Len(t, addresses, 1)
		require.Contains(t, addresses, address1)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.RelayRepository.Save(ctx, event2.Id(), address2)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		addresses, err := adapters.RelayRepository.List(ctx)
		require.NoError(t, err)
		require.Len(t, addresses, 2)
		require.Contains(t, addresses, address1)
		require.Contains(t, addresses, address2)

		return nil
	})
	require.NoError(t, err)
}

func TestRelayRepository_SavingSameDataTwiceDoesNotCreateDuplicates(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	event := fixtures.SomeEvent()
	address := fixtures.SomeMaybeRelayAddress()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.RelayRepository.Save(ctx, event.Id(), address)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		addresses, err := adapters.RelayRepository.List(ctx)
		require.NoError(t, err)
		require.Len(t, addresses, 1)
		require.Contains(t, addresses, address)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.RelayRepository.Save(ctx, event.Id(), address)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		addresses, err := adapters.RelayRepository.List(ctx)
		require.NoError(t, err)
		require.Len(t, addresses, 1)
		require.Contains(t, addresses, address)

		return nil
	})
	require.NoError(t, err)
}
