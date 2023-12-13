package downloader

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const (
	sendOutTasksEvery = 10 * time.Millisecond

	initialWindowAge = 1 * time.Hour
	windowSize       = 1 * time.Minute

	timeWindowTaskConcurrency = 1
)

type CurrentTimeProvider interface {
	GetCurrentTime() time.Time
}

type Task interface {
	Ctx() context.Context
	Filter() domain.Filter

	OnReceivedEOSE()
	OnError(err error)
}

type Scheduler interface {
	GetTasks(ctx context.Context, relay domain.RelayAddress) <-chan Task
}

type TaskScheduler struct {
	taskSubscriptions   []*taskSubscription
	taskGenerators      map[domain.RelayAddress]*RelayTaskGenerator
	lock                sync.Mutex
	parentCtx           context.Context
	publicKeySource     PublicKeySource
	currentTimeProvider CurrentTimeProvider
	logger              logging.Logger
}

func NewTaskScheduler(
	parentCtx context.Context,
	publicKeySource PublicKeySource,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) *TaskScheduler {
	return &TaskScheduler{
		parentCtx:           parentCtx,
		taskGenerators:      make(map[domain.RelayAddress]*RelayTaskGenerator),
		publicKeySource:     publicKeySource,
		currentTimeProvider: currentTimeProvider,
		logger:              logger.New("taskScheduler"),
	}
}

func (t *TaskScheduler) GetTasks(ctx context.Context, relay domain.RelayAddress) <-chan Task {
	taskSubscription := newTaskSubscription(ctx, relay)
	t.addTaskSubscription(taskSubscription)
	return taskSubscription.ch
}

func (t *TaskScheduler) Run(ctx context.Context) error {
	for {
		hadTasks, err := t.sendOutTasks()
		if err != nil {
			return errors.Wrap(err, "error sending out generators")
		}

		if hadTasks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				continue
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sendOutTasksEvery):
			continue
		}
	}
}

func (t *TaskScheduler) sendOutTasks() (bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	sentTasksForAtLeastOneSubscription := false

	for _, taskSubscription := range t.taskSubscriptions {
		sentTasks, err := t.sendOutTasksForSubscription(taskSubscription)
		if err != nil {
			return false, errors.Wrap(err, "error sending out generators")
		}
		if sentTasks {
			sentTasksForAtLeastOneSubscription = true
		}
	}

	return sentTasksForAtLeastOneSubscription, nil
}

func (t *TaskScheduler) sendOutTasksForSubscription(subscription *taskSubscription) (bool, error) {
	generator, err := t.getOrCreateGenerator(subscription.address)
	if err != nil {
		return false, errors.Wrap(err, "error getting a generator")
	}
	// todo cleanup subs
	n, err := generator.PushTasks(subscription.ctx, subscription.ch)
	if err != nil {
		return false, errors.Wrap(err, "error pushing tasks")
	}

	return n > 0, nil
}

func (t *TaskScheduler) getOrCreateGenerator(address domain.RelayAddress) (*RelayTaskGenerator, error) {
	v, ok := t.taskGenerators[address]
	if ok {
		return v, nil
	}

	v, err := NewRelayTaskGenerator(t.parentCtx, t.publicKeySource, t.currentTimeProvider, t.logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating a task generator")
	}
	t.taskGenerators[address] = v
	return v, nil
}

func (t *TaskScheduler) addTaskSubscription(taskSubscription *taskSubscription) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.taskSubscriptions = append(t.taskSubscriptions, taskSubscription)
}

type taskSubscription struct {
	ctx     context.Context
	ch      chan Task
	address domain.RelayAddress
}

func newTaskSubscription(ctx context.Context, address domain.RelayAddress) *taskSubscription {
	return &taskSubscription{
		ctx:     ctx,
		ch:      make(chan Task),
		address: address,
	}
}

type RelayTaskGenerator struct {
	lock sync.Mutex

	globalTask *TimeWindowTaskGenerator
	authorTask *TimeWindowTaskGenerator
	tagTask    *TimeWindowTaskGenerator

	publicKeySource PublicKeySource
	logger          logging.Logger
}

func NewRelayTaskGenerator(
	taskCtx context.Context,
	publicKeySource PublicKeySource,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*RelayTaskGenerator, error) {
	globalTask, err := NewTimeWindowTaskGenerator(
		taskCtx,
		globalEventKindsToDownload,
		nil,
		nil,
		currentTimeProvider,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the global task")
	}
	authorTask, err := NewTimeWindowTaskGenerator(
		taskCtx,
		nil,
		nil,
		nil,
		currentTimeProvider,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the author task")
	}
	tagTask, err := NewTimeWindowTaskGenerator(
		taskCtx,
		nil,
		nil,
		nil,
		currentTimeProvider,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the tag task")
	}

	return &RelayTaskGenerator{
		publicKeySource: publicKeySource,
		globalTask:      globalTask,
		authorTask:      authorTask,
		tagTask:         tagTask,
		logger:          logger.New("relayTaskGenerator"),
	}, nil
}

func (t *RelayTaskGenerator) PushTasks(ctx context.Context, ch chan<- Task) (int, error) {
	tasks, err := t.getTasksToPush(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error getting tasks to push")
	}

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case ch <- task:
			continue
		}
	}
	return len(tasks), nil
}

func (t *RelayTaskGenerator) getTasksToPush(ctx context.Context) ([]Task, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.updateFilters(ctx); err != nil {
		return nil, errors.Wrap(err, "error updating filters")
	}

	var result []Task
	for _, generator := range t.generators() {
		tasks, err := generator.Generate()
		if err != nil {
			return nil, errors.Wrap(err, "error calling one of the generators")
		}
		result = append(result, tasks...)
	}
	return result, nil
}

func (t *RelayTaskGenerator) generators() []*TimeWindowTaskGenerator {
	return []*TimeWindowTaskGenerator{t.globalTask, t.authorTask, t.tagTask}
}

func (t *RelayTaskGenerator) updateFilters(ctx context.Context) error {
	publicKeys, err := t.publicKeySource.GetPublicKeys(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting public keys")
	}

	var pTags []domain.FilterTag
	for _, publicKey := range publicKeys.PublicKeysToMonitor() {
		tag, err := domain.NewFilterTag(domain.TagProfile, publicKey.Hex())
		if err != nil {
			return errors.Wrap(err, "error creating a filter tag")
		}
		pTags = append(pTags, tag)
	}

	t.authorTask.UpdateAuthors(publicKeys.All())
	t.tagTask.UpdateTags(pTags)
	return nil
}

type TimeWindowTaskGenerator struct {
	kinds   []domain.EventKind
	tags    []domain.FilterTag
	authors []domain.PublicKey

	lastWindow             TimeWindow
	runningTimeWindowTasks []*createdTimeWindowTask
	lock                   sync.Mutex

	taskCtx             context.Context
	currentTimeProvider CurrentTimeProvider
}

func NewTimeWindowTaskGenerator(
	taskCtx context.Context,
	kinds []domain.EventKind,
	tags []domain.FilterTag,
	authors []domain.PublicKey,
	currentTimeProvider CurrentTimeProvider,
) (*TimeWindowTaskGenerator, error) {
	now := currentTimeProvider.GetCurrentTime()

	startingWindow, err := NewTimeWindow(now.Add(-initialWindowAge-windowSize), windowSize)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the starting time window")
	}

	return &TimeWindowTaskGenerator{
		taskCtx:             taskCtx,
		lastWindow:          startingWindow,
		kinds:               kinds,
		tags:                tags,
		authors:             authors,
		currentTimeProvider: currentTimeProvider,
	}, nil
}

func (t *TimeWindowTaskGenerator) Generate() ([]Task, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.runningTimeWindowTasks = slices.DeleteFunc(t.runningTimeWindowTasks, func(task *createdTimeWindowTask) bool {
		return task.CheckIfDoneAndEnd()
	})

	for i := len(t.runningTimeWindowTasks); i < timeWindowTaskConcurrency; i++ {
		task, ok := t.generateNewTask()
		if ok {
			t.runningTimeWindowTasks = append(t.runningTimeWindowTasks, task)
		}
	}

	var result []Task
	for _, task := range t.runningTimeWindowTasks {
		t, ok, err := task.MaybeReset(t.taskCtx)
		if err != nil {
			return nil, errors.Wrap(err, "error resetting a task")
		}
		if ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func (t *TimeWindowTaskGenerator) UpdateTags(tags []domain.FilterTag) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.tags = tags
}

func (t *TimeWindowTaskGenerator) UpdateAuthors(authors []domain.PublicKey) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.authors = authors
}

func (t *TimeWindowTaskGenerator) generateNewTask() (*createdTimeWindowTask, bool) {
	nextWindow := t.lastWindow.Advance()
	now := t.currentTimeProvider.GetCurrentTime()
	if nextWindow.End().After(now.Add(-time.Minute)) {
		return nil, false
	}
	t.lastWindow = nextWindow
	return newCreatedTimeWindowTask(t.kinds, t.tags, t.authors, nextWindow), true
}

type createdTimeWindowTask struct {
	kinds   []domain.EventKind
	tags    []domain.FilterTag
	authors []domain.PublicKey
	window  TimeWindow

	task *TimeWindowTask
}

func newCreatedTimeWindowTask(
	kinds []domain.EventKind,
	tags []domain.FilterTag,
	authors []domain.PublicKey,
	window TimeWindow,
) *createdTimeWindowTask {
	return &createdTimeWindowTask{
		kinds:   kinds,
		tags:    tags,
		authors: authors,
		window:  window,
	}
}

func (t *createdTimeWindowTask) CheckIfDoneAndEnd() bool {
	if t.task == nil {
		return false
	}
	if t.task.State() == TimeWindowTaskStateDone {
		t.task.End()
		return true
	}
	return false
}

func (t *createdTimeWindowTask) MaybeReset(taskCtx context.Context) (Task, bool, error) {
	if t.task == nil || t.task.State() != TimeWindowTaskStateStarted {
		task, err := NewTimeWindowTask(taskCtx, t.kinds, t.tags, t.authors, t.window)
		if err != nil {
			return nil, false, errors.Wrap(err, "error creating a new time window task")
		}

		if t.task != nil {
			t.task.End()
		}

		t.task = task
		return task, true, nil
	}
	return nil, false, nil
}
