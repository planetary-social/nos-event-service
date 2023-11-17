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

func TestPublicKeysToMonitorRepository_GetReturnsPredefinedError(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		_, err := adapters.PublicKeysToMonitorRepository.Get(ctx, fixtures.SomePublicKey())
		require.ErrorIs(t, err, app.ErrPublicKeyToMonitorNotFound)

		return nil
	})
	require.NoError(t, err)
}

func TestPublicKeysToMonitorRepository_ListReturnsNoDataWhenRepositoryIsEmpty(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		result, err := adapters.PublicKeysToMonitorRepository.List(ctx)
		require.NoError(t, err)
		require.Empty(t, result)

		return nil
	})
	require.NoError(t, err)
}

func TestPublicKeysToMonitorRepository_GetReturnsData(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	publicKey := fixtures.SomePublicKey()
	publicKeyTime := date(2023, time.November, 1)
	publicKeyToMonitor := domain.MustNewPublicKeyToMonitor(publicKey, publicKeyTime, publicKeyTime)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.PublicKeysToMonitorRepository.Save(ctx, publicKeyToMonitor)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		result, err := adapters.PublicKeysToMonitorRepository.Get(ctx, publicKey)
		require.NoError(t, err)

		kindaEqual(t, publicKeyToMonitor, result)

		return nil
	})
	require.NoError(t, err)
}

func TestPublicKeysToMonitorRepository_SaveUpdatesData(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	publicKey1 := fixtures.SomePublicKey()
	publicKey1Time := date(2023, time.November, 1)
	publicKeyToMonitor1 := domain.MustNewPublicKeyToMonitor(publicKey1, publicKey1Time, publicKey1Time)

	publicKey2 := fixtures.SomePublicKey()
	publicKey2Time1 := date(2023, time.November, 2)
	publicKey2Time2 := date(2023, time.November, 3)
	publicKeyToMonitor21 := domain.MustNewPublicKeyToMonitor(publicKey2, publicKey2Time1, publicKey2Time1)
	publicKeyToMonitor22 := domain.MustNewPublicKeyToMonitor(publicKey2, publicKey2Time2, publicKey2Time2)

	cmp := func(a, b domain.PublicKeyToMonitor) int {
		return strings.Compare(a.PublicKey().Hex(), b.PublicKey().Hex())
	}

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.PublicKeysToMonitorRepository.Save(ctx, publicKeyToMonitor1)
		require.NoError(t, err)

		err = adapters.PublicKeysToMonitorRepository.Save(ctx, publicKeyToMonitor21)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		result, err := adapters.PublicKeysToMonitorRepository.List(ctx)
		require.NoError(t, err)

		expected := []domain.PublicKeyToMonitor{
			domain.MustNewPublicKeyToMonitor(publicKey1, publicKey1Time, publicKey1Time),
			domain.MustNewPublicKeyToMonitor(publicKey2, publicKey2Time1, publicKey2Time1),
		}

		slices.SortFunc(result, cmp)
		slices.SortFunc(expected, cmp)

		kindaEqualSlice(t, expected, result)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.PublicKeysToMonitorRepository.Save(ctx, publicKeyToMonitor22)
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)

	err = adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		result, err := adapters.PublicKeysToMonitorRepository.List(ctx)
		require.NoError(t, err)

		expected := []domain.PublicKeyToMonitor{
			domain.MustNewPublicKeyToMonitor(publicKey1, publicKey1Time, publicKey1Time),
			domain.MustNewPublicKeyToMonitor(publicKey2, publicKey2Time1, publicKey2Time2),
		}

		slices.SortFunc(result, cmp)
		slices.SortFunc(expected, cmp)

		kindaEqualSlice(t, expected, result)

		return nil
	})
	require.NoError(t, err)
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func kindaEqualSlice(tb testing.TB, a, b []domain.PublicKeyToMonitor) {
	require.Equal(tb, len(a), len(b))
	for i := 0; i < len(a); i++ {
		kindaEqual(tb, a[i], b[i])
	}
}

func kindaEqual(tb testing.TB, a, b domain.PublicKeyToMonitor) {
	require.Equal(tb, a.PublicKey(), b.PublicKey())
	require.Equal(tb, a.CreatedAt().Unix(), b.CreatedAt().Unix())
	require.Equal(tb, a.UpdatedAt().Unix(), b.UpdatedAt().Unix())
}
