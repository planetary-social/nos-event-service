package downloader_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
	"github.com/stretchr/testify/require"
)

const numberOfTaskTypes = 3

func TestTaskScheduler_SchedulerWaitsForTasksToCompleteBeforeProducingMore(t *testing.T) {
	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), 5*time.Second)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)

	ch := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())

	var filters []domain.Filter
forloop:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case v := <-ch:
			filters = append(filters, v.Filter())
		case <-time.After(1 * time.Second):
			t.Log("no new tasks for a short while, assuming that scheduler is waiting for them to complete")
			break forloop
		}
	}

	require.Equal(t, numberOfTaskTypes, len(filters))
}

func TestTaskScheduler_SchedulerProducesTasksFromSequentialTimeWindowsLeadingUpToCurrentTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), 5*time.Second)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)

	ch := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())

	filters := make(map[downloader.TimeWindow][]domain.Filter)
forloop:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case v := <-ch:
			start := v.Filter().Since()
			duration := v.Filter().Until().Sub(*v.Filter().Since())
			window := downloader.MustNewTimeWindow(*start, duration)
			filters[window] = append(filters[window], v.Filter())
			v.OnReceivedEOSE()
		case <-time.After(1 * time.Second):
			t.Log("no new tasks for a short while, assuming that scheduler is waiting for them to complete")
			break forloop
		}
	}

	firstWindowStart := date(2023, time.December, 27, 9, 30, 00)
	var expectedWindows []downloader.TimeWindow
	for i := 0; i < 59; i++ {
		window := downloader.MustNewTimeWindow(firstWindowStart.Add(time.Duration(i)*time.Minute), 1*time.Minute)
		expectedWindows = append(expectedWindows, window)
	}

	var windows []downloader.TimeWindow
	for window, filters := range filters {
		require.Equal(t, numberOfTaskTypes, len(filters))
		windows = append(windows, window)
	}

	cmp := func(a, b downloader.TimeWindow) int {
		return a.Start().Compare(b.Start())
	}

	slices.SortFunc(expectedWindows, cmp)
	slices.SortFunc(windows, cmp)
	requireEqualWindows(t, expectedWindows, windows)
}

type testedTaskScheduler struct {
	Scheduler           *downloader.TaskScheduler
	CurrentTimeProvider *mocks.CurrentTimeProvider
}

func newTestedTaskScheduler(ctx context.Context, tb testing.TB) *testedTaskScheduler {
	currentTimeProvider := mocks.NewCurrentTimeProvider()
	source := newMockPublicKeySource()
	logger := logging.NewDevNullLogger()
	scheduler := downloader.NewTaskScheduler(source, currentTimeProvider, logger)
	go func() {
		_ = scheduler.Run(ctx)
	}()

	return &testedTaskScheduler{
		Scheduler:           scheduler,
		CurrentTimeProvider: currentTimeProvider,
	}
}

type mockPublicKeySource struct {
	publicKeys downloader.PublicKeys
}

func newMockPublicKeySource() *mockPublicKeySource {
	return &mockPublicKeySource{
		publicKeys: downloader.NewPublicKeys(nil, nil),
	}
}

func (p *mockPublicKeySource) GetPublicKeys(ctx context.Context) (downloader.PublicKeys, error) {
	return p.publicKeys, nil
}

func date(year int, month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

func requireEqualWindows(tb testing.TB, a, b []downloader.TimeWindow) {
	require.Equal(tb, len(a), len(b))
	for i := 0; i < len(a); i++ {
		require.True(tb, a[i].Start().Equal(b[i].Start()))
		require.True(tb, a[i].End().Equal(b[i].End()))
	}
}
