package sqlite_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestContactRepository_GetCurrentContactsEventReturnsPredefinedError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		_, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, fixtures.SomePublicKey())
		require.ErrorIs(t, err, app.ErrNoContactsEvent)

		return nil
	})
	require.NoError(t, err)
}

func TestContactRepository_GetFollowwesReturnsEmptyListWhenThereIsNoData(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		result, err := adapters.ContactRepository.GetFollowees(ctx, fixtures.SomePublicKey())
		require.NoError(t, err)
		require.Empty(t, result)

		return nil
	})
	require.NoError(t, err)
}

func TestContactRepository_ContactsAreReplacedForGivenPublicKey(t *testing.T) {
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

	cmp := func(a, b domain.PublicKey) int {
		return strings.Compare(a.Hex(), b.Hex())
	}

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		current1, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, event1.Id(), current1.Id())

		current2, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, event2.Id(), current2.Id())

		followees, err := adapters.ContactRepository.GetFollowees(ctx, pk1)
		require.NoError(t, err)

		expected := []domain.PublicKey{
			followee11,
			followee12,
		}

		slices.SortFunc(followees, cmp)
		slices.SortFunc(expected, cmp)
		require.Equal(t, expected, followees)

		followees, err = adapters.ContactRepository.GetFollowees(ctx, pk2)
		require.NoError(t, err)

		expected = []domain.PublicKey{
			followee21,
			followee22,
		}

		slices.SortFunc(followees, cmp)
		slices.SortFunc(expected, cmp)
		require.Equal(t, expected, followees)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.ContactRepository.SetContacts(ctx, event3, []domain.PublicKey{followee31, followee32})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		current1, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, event1.Id(), current1.Id())

		current2, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, event3.Id(), current2.Id())

		followees, err := adapters.ContactRepository.GetFollowees(ctx, pk1)
		require.NoError(t, err)

		expected := []domain.PublicKey{
			followee11,
			followee12,
		}

		slices.SortFunc(followees, cmp)
		slices.SortFunc(expected, cmp)
		require.Equal(t, expected, followees)

		followees, err = adapters.ContactRepository.GetFollowees(ctx, pk2)
		require.NoError(t, err)

		expected = []domain.PublicKey{
			followee31,
			followee32,
		}

		slices.SortFunc(followees, cmp)
		slices.SortFunc(expected, cmp)
		require.Equal(t, expected, followees)

		return nil
	})
	require.NoError(t, err)
}

func TestContactRepository_GrabbingAnEventForFolloweesMeansTheyAreInPublicKeysButNotInContactEventsWhichShouldReturnAKnownError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	pk1, sk1 := fixtures.SomeKeyPair()
	event := fixtures.SomeEventWithAuthor(sk1)
	followee1 := fixtures.SomePublicKey()
	followee2 := fixtures.SomePublicKey()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.ContactRepository.SetContacts(ctx, event, []domain.PublicKey{followee1, followee2})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		currentFollowerEvent, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, event.Id(), currentFollowerEvent.Id())

		_, err = adapters.ContactRepository.GetCurrentContactsEvent(ctx, followee1)
		require.ErrorIs(t, err, app.ErrNoContactsEvent)

		_, err = adapters.ContactRepository.GetCurrentContactsEvent(ctx, followee2)
		require.ErrorIs(t, err, app.ErrNoContactsEvent)

		return nil
	})
	require.NoError(t, err)
}

func TestContactRepository_IsFolloweeOfMonitoredPublicKey(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	pk1, sk1 := fixtures.SomeKeyPair()
	event1 := fixtures.SomeEventWithAuthor(sk1)
	followee11 := fixtures.SomePublicKey()
	followee12 := fixtures.SomePublicKey()
	publicKeyToMonitor := domain.MustNewPublicKeyToMonitor(pk1, time.Now(), time.Now())

	_, sk2 := fixtures.SomeKeyPair()
	event2 := fixtures.SomeEventWithAuthor(sk2)
	followee21 := fixtures.SomePublicKey()
	followee22 := fixtures.SomePublicKey()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)

		err = adapters.EventRepository.Save(ctx, event2)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.PublicKeysToMonitorRepository.Save(ctx, publicKeyToMonitor)
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event1, []domain.PublicKey{followee11, followee12})
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event2, []domain.PublicKey{followee21, followee22})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		ok, err := adapters.ContactRepository.IsFolloweeOfMonitoredPublicKey(ctx, followee11)
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = adapters.ContactRepository.IsFolloweeOfMonitoredPublicKey(ctx, followee12)
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = adapters.ContactRepository.IsFolloweeOfMonitoredPublicKey(ctx, followee21)
		require.NoError(t, err)
		require.False(t, ok)

		ok, err = adapters.ContactRepository.IsFolloweeOfMonitoredPublicKey(ctx, followee22)
		require.NoError(t, err)
		require.False(t, ok)

		ok, err = adapters.ContactRepository.IsFolloweeOfMonitoredPublicKey(ctx, fixtures.SomePublicKey())
		require.NoError(t, err)
		require.False(t, ok)

		return nil
	})
	require.NoError(t, err)
}

func BenchmarkContactRepository_GetCurrentContactsEvent(b *testing.B) {
	ctx := fixtures.TestContext(b)
	adapters := NewTestAdapters(ctx, b)

	eventToLookUp := fixtures.SomeEvent()

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		for i := 0; i < 1000; i++ {
			event := fixtures.SomeEvent()

			err := adapters.EventRepository.Save(ctx, event)
			require.NoError(b, err)
		}

		for i := 0; i < 100; i++ {
			event := fixtures.SomeEvent()

			err := adapters.EventRepository.Save(ctx, event)
			require.NoError(b, err)

			err = adapters.ContactRepository.SetContacts(ctx, event, []domain.PublicKey{fixtures.SomePublicKey(), fixtures.SomePublicKey()})
			require.NoError(b, err)
		}

		err := adapters.EventRepository.Save(ctx, eventToLookUp)
		require.NoError(b, err)

		err = adapters.ContactRepository.SetContacts(ctx, eventToLookUp, []domain.PublicKey{fixtures.SomePublicKey(), fixtures.SomePublicKey()})
		require.NoError(b, err)

		return nil
	})
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
			event, err := adapters.ContactRepository.GetCurrentContactsEvent(ctx, eventToLookUp.PubKey())
			require.NoError(b, err)
			require.Equal(b, eventToLookUp.Id(), event.Id())

			return nil
		})
		require.NoError(b, err)
	}
}

func TestContactRepository_CountFolloweesReturnsNumberOfFollowees(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	pk1, sk1 := fixtures.SomeKeyPair()
	event1 := fixtures.SomeEventWithAuthor(sk1)
	followee11 := fixtures.SomePublicKey()
	followee12 := fixtures.SomePublicKey()

	pk2, sk2 := fixtures.SomeKeyPair()
	event2 := fixtures.SomeEventWithAuthor(sk2)

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		n, err := adapters.ContactRepository.CountFollowees(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		n, err = adapters.ContactRepository.CountFollowees(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)

		err = adapters.EventRepository.Save(ctx, event2)
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event1, []domain.PublicKey{followee11, followee12})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		n, err := adapters.ContactRepository.CountFollowees(ctx, pk1)
		require.NoError(t, err)
		require.Equal(t, 2, n)

		n, err = adapters.ContactRepository.CountFollowees(ctx, pk2)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		return nil
	})
	require.NoError(t, err)
}

func TestContactRepository_CountFollowersReturnsNumberOfFollowers(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	_, sk1 := fixtures.SomeKeyPair()
	event1 := fixtures.SomeEventWithAuthor(sk1)

	_, sk2 := fixtures.SomeKeyPair()
	event2 := fixtures.SomeEventWithAuthor(sk2)

	followee1 := fixtures.SomePublicKey()
	followee2 := fixtures.SomePublicKey()
	followee3 := fixtures.SomePublicKey()

	err := adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		n, err := adapters.ContactRepository.CountFollowers(ctx, followee1)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		n, err = adapters.ContactRepository.CountFollowers(ctx, followee2)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		n, err = adapters.ContactRepository.CountFollowers(ctx, followee3)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.EventRepository.Save(ctx, event1)
		require.NoError(t, err)

		err = adapters.EventRepository.Save(ctx, event2)
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event1, []domain.PublicKey{followee1, followee2})
		require.NoError(t, err)

		err = adapters.ContactRepository.SetContacts(ctx, event2, []domain.PublicKey{followee2})
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		n, err := adapters.ContactRepository.CountFollowers(ctx, followee1)
		require.NoError(t, err)
		require.Equal(t, 1, n)

		n, err = adapters.ContactRepository.CountFollowers(ctx, followee2)
		require.NoError(t, err)
		require.Equal(t, 2, n)

		n, err = adapters.ContactRepository.CountFollowers(ctx, followee3)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		return nil
	})
	require.NoError(t, err)
}
