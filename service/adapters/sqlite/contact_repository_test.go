package sqlite_test

import (
	"context"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestEventRepository_GetCurrentContactsEventReturnsPredefinedError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		_, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, fixtures.SomePublicKey())
		require.ErrorIs(t, err, app.ErrNoContactsEvent)

		return nil
	})
	require.NoError(t, err)
}

func TestEventRepository_ContactsAreReplacesForGivenPublicKey(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	pk1, sk1 := fixtures.SomeKeyPair()
	event1 := fixtures.SomeEventWithAuthor(sk1)
	followee11 := fixtures.SomePublicKey()
	followee12 := fixtures.SomePublicKey()

	pk2, sk2 := fixtures.SomeKeyPair()
	event2 := fixtures.SomeEventWithAuthor(sk2)
	followee21 := fixtures.SomePublicKey()
	followee22 := fixtures.SomePublicKey()
	event3 := fixtures.SomeEventWithAuthor(sk2)
	followee31 := fixtures.SomePublicKey()
	followee32 := fixtures.SomePublicKey()

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

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.ContactRepository.SetContacts(ctx, event1, []domain.PublicKey{followee11, followee12})
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event2, []domain.PublicKey{followee21, followee22})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		current1, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, event1.Id(), current1.Id())

		current2, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, event2.Id(), current2.Id())

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.ContactRepository.SetContacts(ctx, event3, []domain.PublicKey{followee31, followee32})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		current1, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, event1.Id(), current1.Id())

		current2, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, event3.Id(), current2.Id())

		return nil
	})
	require.NoError(t, err)
}
