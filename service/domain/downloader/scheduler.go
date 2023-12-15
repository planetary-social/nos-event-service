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
	sendOutTasksEvery = 100 * time.Millisecond

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
		hadTasks, err := t.sendOutTasks(ctx)
		if err != nil {
			t.logger.Error().WithError(err).Message("error sending out tasks")
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

func (t *TaskScheduler) sendOutTasks(ctx context.Context) (bool, error) {
	publicKeys, err := t.getPublicKeysToReplicate(ctx)
	if err != nil {
		return false, errors.Wrap(err, "error getting public keys to replicate")
	}

	t.taskGeneratorsLock.Lock()
	defer t.taskGeneratorsLock.Unlock()

	atLeastOneHadTasks := false
	for _, taskGenerator := range t.taskGenerators {
		hadTasks, err := taskGenerator.SendOutTasks(publicKeys)
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

	v, err := NewRelayTaskGenerator(t.currentTimeProvider, t.logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating a task generator")
	}
	t.taskGenerators[address] = v
	return v, nil
}

func (t *TaskScheduler) getPublicKeysToReplicate(ctx context.Context) (*PublicKeysToReplicate, error) {
	publicKeys, err := t.publicKeySource.GetPublicKeys(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting public keys")
	}

	return NewPublicKeysToReplicate(publicKeys.All(), publicKeys.PublicKeysToMonitor()), nil
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

	logger logging.Logger
}

func NewRelayTaskGenerator(
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*RelayTaskGenerator, error) {
	globalTask, err := NewTimeWindowTaskGenerator(
		globalEventKindsToDownload,
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the global task")
	}
	authorTask, err := NewTimeWindowTaskGenerator(
		nil,
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the author task")
	}
	tagTask, err := NewTimeWindowTaskGenerator(
		nil,
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the tag task")
	}

	return &RelayTaskGenerator{
		globalTask: globalTask,
		authorTask: authorTask,
		tagTask:    tagTask,
		logger:     logger.New("relayTaskGenerator"),
	}, nil
}

func (t *RelayTaskGenerator) AddSubscription(ctx context.Context, ch chan Task) {
	t.lock.Lock()
	defer t.lock.Unlock()

	taskSubscription := newTaskSubscription(ctx, ch)
	t.taskSubscriptions = append(t.taskSubscriptions, taskSubscription)
}

func (t *RelayTaskGenerator) SendOutTasks(publicKeys *PublicKeysToReplicate) (bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var pTags []domain.FilterTag
	for _, publicKey := range publicKeys.Tagged() {
		tag, err := domain.NewFilterTag(domain.TagProfile, publicKey.Hex())
		if err != nil {
			return false, errors.Wrap(err, "error creating a filter tag")
		}
		pTags = append(pTags, tag)
	}

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
		numberOfSentTasks, err := t.pushTasks(taskSubscription.ctx, taskSubscription.ch, publicKeys.Authors(), pTags)
		if err != nil {
			return false, errors.Wrap(err, "error sending out generators")
		}
		if numberOfSentTasks > 0 {
			sentTasksForAtLeastOneSubscription = true
		}
	}

	return sentTasksForAtLeastOneSubscription, nil
}

func (t *RelayTaskGenerator) pushTasks(ctx context.Context, ch chan<- Task, authors []domain.PublicKey, tags []domain.FilterTag) (int, error) {
	tasks, err := t.getTasksToPush(ctx, authors, tags)
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

func (t *RelayTaskGenerator) getTasksToPush(ctx context.Context, authors []domain.PublicKey, tags []domain.FilterTag) ([]Task, error) {
	var result []Task
	for _, generator := range t.generators(authors, tags) {
		tasks, err := generator.Generate(ctx, authors, tags)
		if err != nil {
			return nil, errors.Wrap(err, "error calling one of the generators")
		}
		result = append(result, tasks...)
	}
	return result, nil
}

func (t *RelayTaskGenerator) generators(authors []domain.PublicKey, tags []domain.FilterTag) []*TimeWindowTaskGenerator {
	generators := []*TimeWindowTaskGenerator{t.globalTask}

	if len(authors) > 0 {
		generators = append(generators, t.authorTask)
	}

	if len(tags) > 0 {
		generators = append(generators, t.tagTask)
	}

	return generators
}

type TimeWindowTaskGenerator struct {
	lastWindow   TimeWindow
	taskTrackers []*TimeWindowTaskTracker
	lock         sync.Mutex
	eventKinds   []domain.EventKind

	currentTimeProvider CurrentTimeProvider
	logger              logging.Logger
}

func NewTimeWindowTaskGenerator(
	eventKinds []domain.EventKind,
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*TimeWindowTaskGenerator, error) {
	now := currentTimeProvider.GetCurrentTime()

	startingWindow, err := NewTimeWindow(now.Add(-initialWindowAge-windowSize), windowSize)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the starting time window")
	}

	return &TimeWindowTaskGenerator{
		eventKinds:          eventKinds,
		lastWindow:          startingWindow,
		currentTimeProvider: currentTimeProvider,
		logger:              logger.New("timeWindowTaskGenerator"),
	}, nil
}

func (t *TimeWindowTaskGenerator) Generate(ctx context.Context, authors []domain.PublicKey, tags []domain.FilterTag) ([]Task, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.taskTrackers = slices.DeleteFunc(t.taskTrackers, func(task *TimeWindowTaskTracker) bool {
		return task.CheckIfDoneAndEnd()
	})

	for i := len(t.taskTrackers); i < timeWindowTaskConcurrency; i++ {
		tracker, ok, err := t.maybeGenerateNewTracker(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "error generating a new task")
		}
		if ok {
			t.taskTrackers = append(t.taskTrackers, tracker)
		}
	}

	var result []Task
	for _, tracker := range t.taskTrackers {
		task, ok, err := tracker.MaybeStart(ctx, t.eventKinds, authors, tags)
		if err != nil {
			return nil, errors.Wrap(err, "error resetting a task")
		}
		if ok {
			result = append(result, task)
		}
	}
	return result, nil
}

func (t *TimeWindowTaskGenerator) maybeGenerateNewTracker(ctx context.Context) (*TimeWindowTaskTracker, bool, error) {
	nextWindow := t.lastWindow.Advance()
	now := t.currentTimeProvider.GetCurrentTime()
	if nextWindow.End().After(now.Add(-time.Minute)) {
		return nil, false, nil
	}
	t.lastWindow = nextWindow
	v, err := NewTimeWindowTaskTracker(nextWindow)
	if err != nil {
		return nil, false, errors.Wrap(err, "error creating a task")
	}
	return v, true, nil
}

type PublicKeysToReplicate struct {
	authors []domain.PublicKey
	tagged  []domain.PublicKey
}

func NewPublicKeysToReplicate(authors []domain.PublicKey, tagged []domain.PublicKey) *PublicKeysToReplicate {
	return &PublicKeysToReplicate{authors: authors, tagged: tagged}
}

func (p PublicKeysToReplicate) Authors() []domain.PublicKey {
	return p.authors
}

func (p PublicKeysToReplicate) Tagged() []domain.PublicKey {
	return p.tagged
}
