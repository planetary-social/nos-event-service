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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	numberOfTaskTypes                           = 3
	testTimeout                                 = 10 * time.Second
	delayWhenWaitingToConsiderThatTasksReceived = 5 * time.Second
)

func TestTaskScheduler_SchedulerWaitsForTasksToCompleteBeforeProducingMore(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), testTimeout)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)
	ts.PublicKeySource.SetPublicKeys(downloader.NewPublicKeys(
		[]domain.PublicKey{fixtures.SomePublicKey()},
		[]domain.PublicKey{fixtures.SomePublicKey()},
	))

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	tasks := collectAllTasks(t, ctx, ch, false)
	require.Equal(t, numberOfTaskTypes, len(tasks))
}

func TestTaskScheduler_SchedulerDoesNotProduceEmptyTasks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), testTimeout)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)
	ts.PublicKeySource.SetPublicKeys(downloader.NewPublicKeys(nil, nil))

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	tasks := collectAllTasks(t, ctx, ch, false)
	require.Equal(t, 1, len(tasks))
}

func TestTaskScheduler_SchedulerProducesTasksFromSequentialTimeWindowsLeadingUpToCurrentTime(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), testTimeout)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)
	ts.PublicKeySource.SetPublicKeys(downloader.NewPublicKeys(
		[]domain.PublicKey{fixtures.SomePublicKey()},
		[]domain.PublicKey{fixtures.SomePublicKey()},
	))

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	tasks := collectAllTasks(t, ctx, ch, true)

	filters := make(map[downloader.TimeWindow][]domain.Filter)
	for _, task := range tasks {
		start := task.Filter().Since()
		duration := task.Filter().Until().Sub(*start)
		window := downloader.MustNewTimeWindow(*start, duration)
		filters[window] = append(filters[window], task.Filter())
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

func TestTaskScheduler_ThereIsOneWindowOfDelayToLetRelaysSyncData(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), testTimeout)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)
	ts.PublicKeySource.SetPublicKeys(downloader.NewPublicKeys(
		nil,
		nil,
	))

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	tasks := collectAllTasks(t, ctx, ch, true)

	var windows []downloader.TimeWindow
	for _, task := range tasks {
		start := task.Filter().Since()
		duration := task.Filter().Until().Sub(*start)
		window := downloader.MustNewTimeWindow(*start, duration)
		windows = append(windows, window)
	}

	slices.SortFunc(windows, func(a, b downloader.TimeWindow) int {
		return a.Start().Compare(b.Start())
	})

	require.Len(t, windows, 59)
	lastWindow := windows[len(windows)-1]
	require.Equal(t, date(2023, time.December, 27, 10, 29, 00), lastWindow.End().UTC())
}

func TestTaskScheduler_TerminatesTasks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(fixtures.TestContext(t), testTimeout)
	defer cancel()

	start := date(2023, time.December, 27, 10, 30, 00)

	ts := newTestedTaskScheduler(ctx, t)
	ts.CurrentTimeProvider.SetCurrentTime(start)

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	firstTaskCh := make(chan downloader.Task)

	go func() {
		first := true
		for {
			select {
			case <-ctx.Done():
				return
			case v := <-ch:
				go v.OnReceivedEOSE()
				if first {
					first = false
					select {
					case firstTaskCh <- v:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	select {
	case v := <-firstTaskCh:
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.Error(collect, v.Ctx().Err())
		}, 5*time.Second, 10*time.Millisecond)
	case <-ctx.Done():
		t.Fatal("timeout")
	}

}

type testedTaskScheduler struct {
	Scheduler           *downloader.TaskScheduler
	CurrentTimeProvider *mocks.CurrentTimeProvider
	PublicKeySource     *mockPublicKeySource
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
		PublicKeySource:     source,
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

func (p *mockPublicKeySource) SetPublicKeys(publicKeys downloader.PublicKeys) {
	p.publicKeys = publicKeys
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

func collectAllTasks(t *testing.T, ctx context.Context, ch <-chan downloader.Task, ackAll bool) []downloader.Task {
	var tasks []downloader.Task
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case task := <-ch:
			if ackAll {
				go task.OnReceivedEOSE()
			}
			tasks = append(tasks, task)
		case <-time.After(delayWhenWaitingToConsiderThatTasksReceived):
			t.Log("no new tasks for a short while, assuming that scheduler is gave us all tasks and waiting for the current ones to be completed")
			return tasks
		}
	}
}
