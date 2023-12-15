package downloader_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
	"github.com/stretchr/testify/require"
)

func TestTimeWindowTaskTracker_ReportingErrorsAfterTaskIsConsideredToBeDoneShouldBeIgnored(t *testing.T) {
	ctx := fixtures.TestContext(t)

	tracker, err := downloader.NewTimeWindowTaskTracker(fixtures.SomeTimeWindow())
	require.NoError(t, err)

	task, ok, err := tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)

	task.OnReceivedEOSE()

	ok = tracker.CheckIfDoneAndEnd()
	require.True(t, ok)

	task.OnError(fixtures.SomeError())

	ok = tracker.CheckIfDoneAndEnd()
	require.True(t, ok)
}

func TestTimeWindowTaskTracker_NewTasksCanBeStarted(t *testing.T) {
	ctx := fixtures.TestContext(t)

	tracker, err := downloader.NewTimeWindowTaskTracker(fixtures.SomeTimeWindow())
	require.NoError(t, err)

	_, ok, err := tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestTimeWindowTaskTracker_FailedTasksCanBeStarted(t *testing.T) {
	ctx := fixtures.TestContext(t)

	tracker, err := downloader.NewTimeWindowTaskTracker(fixtures.SomeTimeWindow())
	require.NoError(t, err)

	task, ok, err := tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)

	task.OnError(fixtures.SomeError())

	_, ok, err = tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestTimeWindowTaskTracker_StartedTasksCanNotBeStarted(t *testing.T) {
	ctx := fixtures.TestContext(t)

	tracker, err := downloader.NewTimeWindowTaskTracker(fixtures.SomeTimeWindow())
	require.NoError(t, err)

	_, ok, err := tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)

	_, ok, err = tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestTimeWindowTaskTracker_DoneTasksCanNotBeStarted(t *testing.T) {
	ctx := fixtures.TestContext(t)

	tracker, err := downloader.NewTimeWindowTaskTracker(fixtures.SomeTimeWindow())
	require.NoError(t, err)

	task, ok, err := tracker.MaybeStart(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)

	task.OnReceivedEOSE()

	_, _, err = tracker.MaybeStart(ctx, nil, nil, nil)
	require.EqualError(t, err, "why are we trying to reset a completed task?")
}
