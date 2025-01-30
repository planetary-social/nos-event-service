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

	initialWindowAge = 60 * time.Minute
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

// A TaskScheduler is responsible for generating tasks for all relays by maintaing a list of TaskGenerators.
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
	// The subscription for the relay will generate a sequence of tasks mapped to time windows
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

// A RelayTaskGenerator is responsible for generating tasks for a single relay.
// Each task provides the time window for each query (since and until) and keeps
// track of how many available running queries we can perform on the relay, we only
// allow 3 running queries at a time; for global, author and tag queries across all
// subscription channels.

// RelayTaskGenerator maintains 3 TimeWindowTaskGenerator, one for each query
// type. Each TimeWindowTaskGenerator maintains a list of TimeWindowTaskTracker,
// one for each time window. Each TimeWindowTaskTracker maintains a list of
// runningRelayDownloader, one for each concurrency setting. Each TimeWindowTaskTracker uses a TimeWindow
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
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the global task")
	}
	authorTask, err := NewTimeWindowTaskGenerator(
		currentTimeProvider,
		logger,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the author task")
	}
	tagTask, err := NewTimeWindowTaskGenerator(
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

	// Create tags for the public keys.
	var pTags []domain.FilterTag
	for _, publicKey := range publicKeys.Tagged() {
		tag, err := domain.NewFilterTag(domain.TagProfile, publicKey.Hex())
		if err != nil {
			return false, errors.Wrap(err, "error creating a filter tag")
		}
		pTags = append(pTags, tag)
	}

	// Delete each done subscription.
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
		// Send a task for each subscription
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

// Pushes tasks to the task channel. If tasks are not done nothing is pushed.
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

	tasks, err := t.globalTask.Generate(ctx, globalEventKindsToDownload, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error calling one of the generators")
	}
	result = append(result, tasks...)

	if len(authors) > 0 {
		tasks, err := t.authorTask.Generate(ctx, nil, authors, nil)
		if err != nil {
			return nil, errors.Wrap(err, "error calling one of the generators")
		}
		result = append(result, tasks...)
	}

	if len(tags) > 0 {
		tasks, err := t.tagTask.Generate(ctx, nil, nil, tags)
		if err != nil {
			return nil, errors.Wrap(err, "error calling one of the generators")
		}
		result = append(result, tasks...)
	}

	return result, nil
}

type TimeWindowTaskGenerator struct {
	lastWindow   TimeWindow
	taskTrackers []*TimeWindowTaskTracker
	lock         sync.Mutex

	currentTimeProvider CurrentTimeProvider
	logger              logging.Logger
}

func NewTimeWindowTaskGenerator(
	currentTimeProvider CurrentTimeProvider,
	logger logging.Logger,
) (*TimeWindowTaskGenerator, error) {
	now := currentTimeProvider.GetCurrentTime()

	startingWindow, err := NewTimeWindow(now.Add(-initialWindowAge).Add(-windowSize), windowSize)
	if err != nil {
		return nil, errors.Wrap(err, "error creating the starting time window")
	}

	return &TimeWindowTaskGenerator{
		lastWindow:          startingWindow,
		currentTimeProvider: currentTimeProvider,
		logger:              logger.New("timeWindowTaskGenerator"),
	}, nil
}

// A task generator creates a task tracker per concurrency setting. The tracker
// will be used to return the corresponding task, if the task is still runnning
// it will return no task. If the task is done it will discard the current
// tracker, create a new one and return a new task.
// Each task generated will be pushed to all subscribers of the scheduler
func (t *TimeWindowTaskGenerator) Generate(ctx context.Context, kinds []domain.EventKind, authors []domain.PublicKey, tags []domain.FilterTag) ([]Task, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.taskTrackers = slices.DeleteFunc(t.taskTrackers, func(tracker *TimeWindowTaskTracker) bool {
		return tracker.CheckIfDoneAndEnd()
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
		task, ok, err := tracker.MaybeStart(ctx, kinds, authors, tags)
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
	if nextWindow.End().After(now.Add(-windowSize)) {
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
