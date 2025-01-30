package downloader_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/planetary-social/nos-event-service/internal"
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

	waitFor = 60 * time.Second
	tick    = 100 * time.Millisecond
)

func TestTaskScheduler_SchedulerWaitsForTasksToCompleteBeforeProducingMore(t *testing.T) {
	t.Parallel()

	ctx := fixtures.TestContext(t)
	ts := newTestedTaskScheduler(ctx, t)

	start := date(2023, time.December, 27, 10, 30, 00)
	ts.CurrentTimeProvider.SetCurrentTime(start)
	ts.PublicKeySource.SetPublicKeys(downloader.NewPublicKeys(
		[]domain.PublicKey{fixtures.SomePublicKey()},
		[]domain.PublicKey{fixtures.SomePublicKey()},
	))

	ch, err := ts.Scheduler.GetTasks(ctx, fixtures.SomeRelayAddress())
	require.NoError(t, err)

	completeTasks := false
	tasks := collectAllTasks(ctx, ch, completeTasks)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.Equal(t, numberOfTaskTypes, len(tasks.Tasks()))
	}, waitFor, tick)

	<-time.After(5 * time.Second)
	require.Len(t, tasks.Tasks(), numberOfTaskTypes)
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

	completeTasks := false
	tasks := collectAllTasks(ctx, ch, completeTasks)

	<-time.After(5 * time.Second)

	// No public keys so no task for authors or ptags, but still one for event kinds
	require.Len(t, tasks.Tasks(), 1)
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

	completeTasks := true
	tasks := collectAllTasks(ctx, ch, completeTasks)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		filters := make(map[downloader.TimeWindow][]domain.Filter)
		for _, task := range tasks.Tasks() {
			start := task.Filter().Since()
			duration := task.Filter().Until().Sub(*start)
			window := downloader.MustNewTimeWindow(*start, duration)
			// there will be 3 tasks per window, one for kind filter, one for authors filter and one for tags filter
			filters[window] = append(filters[window], task.Filter())
		}

		firstWindowStart := date(2023, time.December, 27, 10, 14, 00)
		var expectedWindows []downloader.TimeWindow
		for i := 0; i < 15; i++ {
			window := downloader.MustNewTimeWindow(firstWindowStart.Add(time.Duration(i)*time.Minute), 1*time.Minute)
			expectedWindows = append(expectedWindows, window)
		}

		var windows []downloader.TimeWindow
		for window, filters := range filters {
			assert.Equal(t, numberOfTaskTypes, len(filters))
			windows = append(windows, window)
		}

		cmp := func(a, b downloader.TimeWindow) int {
			return a.Start().Compare(b.Start())
		}

		slices.SortFunc(expectedWindows, cmp)
		slices.SortFunc(windows, cmp)
		assertEqualWindows(t, expectedWindows, windows)
	}, waitFor, tick)
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

	tasks := collectAllTasks(ctx, ch, true)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var windows []downloader.TimeWindow
		for _, task := range tasks.Tasks() {
			start := task.Filter().Since()
			duration := task.Filter().Until().Sub(*start)
			window := downloader.MustNewTimeWindow(*start, duration)
			windows = append(windows, window)
		}

		slices.SortFunc(windows, func(a, b downloader.TimeWindow) int {
			return a.Start().Compare(b.Start())
		})

		if assert.Len(t, windows, 15) {
			lastWindow := windows[len(windows)-1]
			assert.Equal(t, date(2023, time.December, 27, 10, 29, 00), lastWindow.End().UTC())
		}
	}, waitFor, tick)

	firstWindowStart := date(2023, time.December, 27, 10, 14, 00)
	var expectedWindows []downloader.TimeWindow
	for i := 0; i < 16; i++ {
		windowStart := firstWindowStart.Add(time.Duration(i) * time.Minute)
		window := downloader.MustNewTimeWindow(windowStart, 1*time.Minute)
		expectedWindows = append(expectedWindows, window)
	}
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
	publicKeys     downloader.PublicKeys
	publicKeysLock sync.Mutex
}

func newMockPublicKeySource() *mockPublicKeySource {
	return &mockPublicKeySource{
		publicKeys: downloader.NewPublicKeys(nil, nil),
	}
}

func (p *mockPublicKeySource) SetPublicKeys(publicKeys downloader.PublicKeys) {
	p.publicKeysLock.Lock()
	defer p.publicKeysLock.Unlock()

	p.publicKeys = publicKeys
}

func (p *mockPublicKeySource) GetPublicKeys(ctx context.Context) (downloader.PublicKeys, error) {
	p.publicKeysLock.Lock()
	defer p.publicKeysLock.Unlock()

	return p.publicKeys, nil
}

func date(year int, month time.Month, day, hour, min, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}

func assertEqualWindows(tb require.TestingT, a, b []downloader.TimeWindow) {
	assert.Equal(tb, len(a), len(b))
	if len(a) == len(b) {
		for i := 0; i < len(a); i++ {
			assert.True(tb, a[i].Start().Equal(b[i].Start()))
			assert.True(tb, a[i].End().Equal(b[i].End()))
		}
	}
}

type collectedTasks struct {
	tasks []downloader.Task
	l     sync.Mutex
}

func newCollectedTasks() *collectedTasks {
	return &collectedTasks{}
}

func (c *collectedTasks) add(task downloader.Task) {
	c.l.Lock()
	defer c.l.Unlock()
	c.tasks = append(c.tasks, task)
}

func (c *collectedTasks) Tasks() []downloader.Task {
	c.l.Lock()
	defer c.l.Unlock()
	return internal.CopySlice(c.tasks)
}

func collectAllTasks(ctx context.Context, ch <-chan downloader.Task, ackAll bool) *collectedTasks {
	v := newCollectedTasks()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-ch:
				if ackAll {
					go task.OnReceivedEOSE()
				}
				v.add(task)
			}
		}
	}()
	return v
}
