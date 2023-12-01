package sqlite_test

import (
	"context"
	"testing"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/cmd/event-service/di"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/stretchr/testify/require"
)

func NewTestAdapters(ctx context.Context, tb testing.TB) sqlite.TestedItems {
	adapters, f, err := di.BuildTestAdapters(ctx, tb)
	require.NoError(tb, err)

	tb.Cleanup(f)

	err = adapters.MigrationsRunner.Run(ctx, adapters.Migrations, adapters.MigrationsProgressCallback)
	require.NoError(tb, err)

	go func() {
		if err := adapters.TransactionRunner.Run(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				panic(err)
			}
		}
	}()

	return adapters
}
