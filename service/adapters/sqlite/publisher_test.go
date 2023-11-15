package sqlite_test

import (
	"context"
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/stretchr/testify/require"
)

func TestPublisher_ItIsPossibleToPublishEvents(t *testing.T) {
	ctx := fixtures.TestContext(t)
	adapters := NewTestAdapters(ctx, t)

	err := adapters.TransactionProvider.Transact(ctx, func(ctx context.Context, adapters sqlite.TestAdapters) error {
		err := adapters.Publisher.PublishEventSaved(ctx, fixtures.SomeEventID())
		require.NoError(t, err)

		return nil
	})
	require.NoError(t, err)
}
