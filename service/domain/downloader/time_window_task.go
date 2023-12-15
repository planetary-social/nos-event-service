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

func NewTimeWindowTaskTracker(
	ctx context.Context,
	window TimeWindow,
) (*TimeWindowTaskTracker, error) {
	ctx, cancel := context.WithCancel(ctx)
	t := &TimeWindowTaskTracker{
		ctx:    ctx,
		cancel: cancel,
		state:  TimeWindowTaskStateNew,
		window: window,
	}
	return t, nil
}

func (t *TimeWindowTaskTracker) MarkAsDone() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.state = TimeWindowTaskStateDone
}

func (t *TimeWindowTaskTracker) MarkAsFailed() {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != TimeWindowTaskStateDone {
		t.state = TimeWindowTaskStateError
	}
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

	t.cancel()

	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel
	t.state = TimeWindowTaskStateStarted

	task := NewTimeWindowTask(filter, t)
	return task, true, nil
}

func (t *TimeWindowTaskTracker) isDead() bool {
	if err := t.ctx.Err(); err != nil {
		return true
	}
	return t.state == TimeWindowTaskStateError || t.state == TimeWindowTaskStateNew
}

type TimeWindowTask struct {
	filter domain.Filter
	t      *TimeWindowTaskTracker
}

func NewTimeWindowTask(filter domain.Filter, t *TimeWindowTaskTracker) *TimeWindowTask {
	return &TimeWindowTask{filter: filter, t: t}
}

func (t *TimeWindowTask) Ctx() context.Context {
	return t.t.ctx
}

func (t *TimeWindowTask) Filter() domain.Filter {
	return t.filter
}

func (t *TimeWindowTask) OnReceivedEOSE() {
	t.t.MarkAsDone()
}

func (t *TimeWindowTask) OnError(err error) {
	t.t.MarkAsFailed()
}
