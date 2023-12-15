package downloader

import (
	"context"
	"sync"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var (
	TimeWindowTaskStateNew     = TimeWindowTaskState{"new"}
	TimeWindowTaskStateStarted = TimeWindowTaskState{"started"}
	TimeWindowTaskStateDone    = TimeWindowTaskState{"done"}
	TimeWindowTaskStateError   = TimeWindowTaskState{"error"}
)

type TimeWindowTaskState struct {
	s string
}

type TimeWindowTaskTracker struct {
	window TimeWindow

	ctx    context.Context
	cancel context.CancelFunc
	state  TimeWindowTaskState
	lock   sync.Mutex
}

func NewTimeWindowTaskTracker(window TimeWindow) (*TimeWindowTaskTracker, error) {
	t := &TimeWindowTaskTracker{
		state:  TimeWindowTaskStateNew,
		window: window,
	}
	return t, nil
}

func (t *TimeWindowTaskTracker) CheckIfDoneAndEnd() bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != TimeWindowTaskStateDone {
		return false
	}

	t.cancel()
	return true
}

func (t *TimeWindowTaskTracker) MaybeStart(ctx context.Context, kinds []domain.EventKind, authors []domain.PublicKey, tags []domain.FilterTag) (Task, bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state == TimeWindowTaskStateDone {
		return nil, false, errors.New("why are we trying to reset a completed task?")
	}

	if !t.isDead() {
		return nil, false, nil
	}

	filter, err := domain.NewFilter(
		nil,
		kinds,
		tags,
		authors,
		internal.Pointer(t.window.Start()),
		internal.Pointer(t.window.End()),
	)
	if err != nil {
		return nil, false, errors.Wrap(err, "error creating a filter")
	}

	if t.cancel != nil {
		t.cancel()
	}

	ctx, cancel := context.WithCancel(ctx)

	t.ctx = ctx
	t.cancel = cancel
	t.state = TimeWindowTaskStateStarted

	task := NewTimeWindowTask(ctx, filter, t)
	return task, true, nil
}

func (t *TimeWindowTaskTracker) markAsDone() {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state == TimeWindowTaskStateStarted {
		t.state = TimeWindowTaskStateDone
	}
}

func (t *TimeWindowTaskTracker) markAsFailed() {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != TimeWindowTaskStateDone {
		t.state = TimeWindowTaskStateError
	}
}

func (t *TimeWindowTaskTracker) isDead() bool {
	return t.ctxIsDead() || t.state == TimeWindowTaskStateError || t.state == TimeWindowTaskStateNew
}

func (t *TimeWindowTaskTracker) ctxIsDead() bool {
	if t.ctx == nil {
		return true
	}
	if err := t.ctx.Err(); err != nil {
		return true
	}
	return false
}

type TimeWindowTask struct {
	ctx     context.Context
	filter  domain.Filter
	tracker tracker
}

func NewTimeWindowTask(ctx context.Context, filter domain.Filter, tracker tracker) *TimeWindowTask {
	return &TimeWindowTask{
		ctx:     ctx,
		filter:  filter,
		tracker: tracker,
	}
}

func (t *TimeWindowTask) Ctx() context.Context {
	return t.ctx
}

func (t *TimeWindowTask) Filter() domain.Filter {
	return t.filter
}

func (t *TimeWindowTask) OnReceivedEOSE() {
	if t.ctx.Err() == nil {
		t.tracker.markAsDone()
	}
}

func (t *TimeWindowTask) OnError(err error) {
	if t.ctx.Err() == nil {
		t.tracker.markAsFailed()
	}
}

type tracker interface {
	markAsDone()
	markAsFailed()
}
