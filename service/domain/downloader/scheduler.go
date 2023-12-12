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
	sendOutTasksEvery = 1 * time.Second

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
	taskSubscriptions []*taskSubscription
	taskGenerators    map[domain.RelayAddress]*RelayTaskGenerator
	lock              sync.Mutex

	publicKeySource     PublicKeySource
	currentTimeProvider CurrentTimeProvider
	logger              logging.Logger
}

func NewTaskScheduler(
	publicKeySource PublicKeySource,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) *TaskScheduler {
	return &TaskScheduler{
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
		if err := t.sendOutTasks(); err != nil {
			return errors.Wrap(err, "error sending out generators")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sendOutTasksEvery):
			continue
		}
	}
}

func (t *TaskScheduler) sendOutTasks() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, taskSubscription := range t.taskSubscriptions {
		if err := t.sendOutTasksForSubscription(taskSubscription); err != nil {
			return errors.Wrap(err, "error sending out generators")
		}
	}

	return nil
}

func (t *TaskScheduler) sendOutTasksForSubscription(subscription *taskSubscription) error {
	generator, err := t.getOrCreateGenerator(subscription.address)
	if err != nil {
		return errors.Wrap(err, "error getting a generator")
	}
	return generator.PushTasks(subscription.ctx, subscription.ch)
}

func (t *TaskScheduler) getOrCreateGenerator(address domain.RelayAddress) (*RelayTaskGenerator, error) {
	v, ok := t.taskGenerators[address]
	if ok {
		return v, nil
	}

	v, err := NewRelayTaskGenerator(t.publicKeySource, t.currentTimeProvider, t.logger)
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
	publicKeySource PublicKeySource,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*RelayTaskGenerator, error) {
	globalTask, err := NewTimeWindowTaskGenerator(
		globalEventKindsToDownload,
		nil,
		nil,
		currentTimeProvider,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the global task")
	}
	authorTask, err := NewTimeWindowTaskGenerator(
		nil,
		nil,
		nil,
		currentTimeProvider,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the author task")
	}
	tagTask, err := NewTimeWindowTaskGenerator(
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

func (t *RelayTaskGenerator) PushTasks(ctx context.Context, ch chan<- Task) error {
	for _, task := range t.getTasksToPush(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- task:
			continue
		}
	}
	return nil
}

func (t *RelayTaskGenerator) getTasksToPush(ctx context.Context) []Task {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.updateFilters(ctx); err != nil {

	}

	var result []Task
	for _, generator := range t.generators() {
		result = append(result, generator.Generate()...)
	}
	return result
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
	createdTimeWindowTasks []*createdTimeWindowTask
	lock                   sync.Mutex

	currentTimeProvider CurrentTimeProvider
}

func NewTimeWindowTaskGenerator(
	kinds []domain.EventKind,
	tags []domain.FilterTag,
	authors []domain.PublicKey,
	currentTimeProvider CurrentTimeProvider,
) (*TimeWindowTaskGenerator, error) {
	now := currentTimeProvider.GetCurrentTime()

	startingWindow, err := NewTimeWindow(now.Add(-initialWindowAge), windowSize)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the starting time window")
	}

	return &TimeWindowTaskGenerator{
		lastWindow:          startingWindow,
		kinds:               kinds,
		tags:                tags,
		authors:             authors,
		currentTimeProvider: currentTimeProvider,
	}, nil
}

func (t *TimeWindowTaskGenerator) Generate() []Task {
	t.lock.Lock()
	defer t.lock.Unlock()

	slices.DeleteFunc(t.createdTimeWindowTasks, func(task *createdTimeWindowTask) bool {
		return task.Done()
	})

	for i := len(t.createdTimeWindowTasks); i < timeWindowTaskConcurrency; i++ {
		task, ok := t.generateNewTask()
		if ok {
			t.createdTimeWindowTasks = append(t.createdTimeWindowTasks, task)
		}
	}

	var result []Task
	for _, task := range t.createdTimeWindowTasks {
		if t, ok := task.Reset(); ok {
			result = append(result, t)
		}
	}
	return result
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

func (t *createdTimeWindowTask) Done() bool {
	if t.task == nil {
		return false
	}
	return t.task.State() == TimeWindowTaskStateDone
}

func (t *createdTimeWindowTask) Reset() (Task, bool) {
	if t.task == nil || t.task.state != TimeWindowTaskStateStarted {
		task, err := NewTimeWindowTask(t.kinds, t.tags, t.authors, t.window)
		if err != nil {
			panic(err) // todo
		}
		t.task = task
		return task, true
	}
	return nil, false
}
