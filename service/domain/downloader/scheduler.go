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
	GetTasks(ctx context.Context, relay domain.RelayAddress) (<-chan Task, error)
}

type TaskScheduler struct {
	taskGeneratorsLock sync.Mutex
	taskGenerators     map[domain.RelayAddress]*RelayTaskGenerator

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

func (t *TaskScheduler) GetTasks(ctx context.Context, relay domain.RelayAddress) (<-chan Task, error) {
	generator, err := t.getOrCreateGeneratorWithLock(relay)
	if err != nil {
		return nil, errors.Wrap(err, "error getting a generator")
	}

	ch := make(chan Task)
	generator.AddSubscription(ctx, ch)
	return ch, nil
}

func (t *TaskScheduler) Run(ctx context.Context) error {
	for {
		hadTasks, err := t.sendOutTasks()
		if err != nil {
			return errors.Wrap(err, "error sending out tasks")
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
	t.taskGeneratorsLock.Lock()
	defer t.taskGeneratorsLock.Unlock()

	atLeastOneHadTasks := false
	for _, taskGenerator := range t.taskGenerators {
		hadTasks, err := taskGenerator.SendOutTasks()
		if err != nil {
			return false, errors.Wrap(err, "error calling task generator")
		}
		if hadTasks {
			atLeastOneHadTasks = true
		}
	}

	return atLeastOneHadTasks, nil
}

func (t *TaskScheduler) getOrCreateGeneratorWithLock(address domain.RelayAddress) (*RelayTaskGenerator, error) {
	t.taskGeneratorsLock.Lock()
	defer t.taskGeneratorsLock.Unlock()

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

type taskSubscription struct {
	ctx context.Context
	ch  chan Task
}

func newTaskSubscription(ctx context.Context, ch chan Task) *taskSubscription {
	return &taskSubscription{
		ctx: ctx,
		ch:  ch,
	}
}

type RelayTaskGenerator struct {
	lock sync.Mutex

	taskSubscriptions []*taskSubscription

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
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the global task")
	}
	authorTask, err := NewTimeWindowTaskGenerator(
		nil,
		nil,
		nil,
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the author task")
	}
	tagTask, err := NewTimeWindowTaskGenerator(
		nil,
		nil,
		nil,
		currentTimeProvider,
		logger,
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

func (t *RelayTaskGenerator) AddSubscription(ctx context.Context, ch chan Task) {
	t.lock.Lock()
	defer t.lock.Unlock()

	taskSubscription := newTaskSubscription(ctx, ch)
	t.taskSubscriptions = append(t.taskSubscriptions, taskSubscription)
}

func (t *RelayTaskGenerator) SendOutTasks() (bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	slices.DeleteFunc(t.taskSubscriptions, func(subscription *taskSubscription) bool {
		select {
		case <-subscription.ctx.Done():
			return true
		default:
			return false
		}
	})

	sentTasksForAtLeastOneSubscription := false
	for _, taskSubscription := range t.taskSubscriptions {
		numberOfSentTasks, err := t.pushTasks(taskSubscription.ctx, taskSubscription.ch)
		if err != nil {
			return false, errors.Wrap(err, "error sending out generators")
		}
		if numberOfSentTasks > 0 {
			sentTasksForAtLeastOneSubscription = true
		}
	}

	return sentTasksForAtLeastOneSubscription, nil
}

func (t *RelayTaskGenerator) pushTasks(ctx context.Context, ch chan<- Task) (int, error) {
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
	if err := t.updateFilters(ctx); err != nil {
		return nil, errors.Wrap(err, "error updating filters")
	}

	var result []Task
	for _, generator := range t.generators() {
		tasks, err := generator.Generate(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error calling one of the generators")
		}
		result = append(result, tasks...)
	}
	return result, nil
}

func (t *RelayTaskGenerator) generators() []*TimeWindowTaskGenerator {
	generators := []*TimeWindowTaskGenerator{t.globalTask}

	if len(t.authorTask.authors) > 0 {
		generators = append(generators, t.authorTask)
	}

	if len(t.tagTask.tags) > 0 {
		generators = append(generators, t.tagTask)
	}

	return generators
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
	runningTimeWindowTasks []*TimeWindowTask
	lock                   sync.Mutex

	currentTimeProvider CurrentTimeProvider
	logger              logging.Logger
}

func NewTimeWindowTaskGenerator(
	kinds []domain.EventKind,
	tags []domain.FilterTag,
	authors []domain.PublicKey,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*TimeWindowTaskGenerator, error) {
	now := currentTimeProvider.GetCurrentTime()

	startingWindow, err := NewTimeWindow(now.Add(-initialWindowAge-windowSize), windowSize)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the starting time window")
	}

	return &TimeWindowTaskGenerator{
		lastWindow:          startingWindow,
		kinds:               kinds,
		tags:                tags,
		authors:             authors,
		currentTimeProvider: currentTimeProvider,
		logger:              logger.New("timeWindowTaskGenerator"),
	}, nil
}

func (t *TimeWindowTaskGenerator) Generate(ctx context.Context) ([]Task, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.runningTimeWindowTasks = slices.DeleteFunc(t.runningTimeWindowTasks, func(task *TimeWindowTask) bool {
		return task.CheckIfDoneAndEnd()
	})

	var result []Task

	for i := len(t.runningTimeWindowTasks); i < timeWindowTaskConcurrency; i++ {
		task, ok, err := t.maybeGenerateNewTask(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error generating a new task")
		}
		if ok {
			t.runningTimeWindowTasks = append(t.runningTimeWindowTasks, task)
			result = append(result, task)
		}
	}

	for _, task := range t.runningTimeWindowTasks {
		ok, err := task.MaybeReset(ctx, t.kinds, t.tags, t.authors)
		if err != nil {
			return nil, errors.Wrap(err, "error resetting a task")
		}
		if ok {
			result = append(result, task)
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

func (t *TimeWindowTaskGenerator) maybeGenerateNewTask(ctx context.Context) (*TimeWindowTask, bool, error) {
	nextWindow := t.lastWindow.Advance()
	now := t.currentTimeProvider.GetCurrentTime()
	if nextWindow.End().After(now.Add(-time.Minute)) {
		return nil, false, nil
	}
	t.lastWindow = nextWindow
	v, err := NewTimeWindowTask(ctx, t.kinds, t.tags, t.authors, nextWindow)
	if err != nil {
		return nil, false, errors.Wrap(err, "error creating a task")
	}
	return v, true, nil
}
