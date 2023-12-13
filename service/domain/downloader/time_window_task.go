package downloader

import (
	"context"
	"sync"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var (
	TimeWindowTaskStateStarted = TimeWindowTaskState{"started"}
	TimeWindowTaskStateDone    = TimeWindowTaskState{"done"}
	TimeWindowTaskStateError   = TimeWindowTaskState{"error"}
)

type TimeWindowTaskState struct {
	s string
}

type TimeWindowTask struct {
	ctx    context.Context
	cancel context.CancelFunc
	filter domain.Filter
	state  TimeWindowTaskState
	lock   sync.Mutex
}

func NewTimeWindowTask(
	ctx context.Context,
	kinds []domain.EventKind,
	tags []domain.FilterTag,
	authors []domain.PublicKey,
	window TimeWindow,
) (*TimeWindowTask, error) {
	filter, err := domain.NewFilter(
		nil,
		kinds,
		tags,
		authors,
		internal.Pointer(window.Start()),
		internal.Pointer(window.End()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating a filter")
	}

	ctx, cancel := context.WithCancel(ctx)

	return &TimeWindowTask{
		ctx:    ctx,
		cancel: cancel,
		filter: filter,
		state:  TimeWindowTaskStateStarted,
	}, nil
}

func (t *TimeWindowTask) Ctx() context.Context {
	return t.ctx
}

func (t *TimeWindowTask) Filter() domain.Filter {
	return t.filter
}

func (t *TimeWindowTask) OnReceivedEOSE() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.state = TimeWindowTaskStateDone
}

func (t *TimeWindowTask) OnError(err error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.state = TimeWindowTaskStateError
}

func (t *TimeWindowTask) CheckIfDoneAndEnd() bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != TimeWindowTaskStateDone {
		return false
	}

	t.cancel()
	return true
}

func (t *TimeWindowTask) MaybeReset(ctx context.Context) (bool, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state == TimeWindowTaskStateDone {
		return false, errors.New("why are we trying to reset a completed task?")
	}

	if !t.isDead() {
		return false, nil
	}

	t.cancel()

	ctx, cancel := context.WithCancel(ctx)
	t.ctx = ctx
	t.cancel = cancel
	return true, nil
}

func (t *TimeWindowTask) isDead() bool {
	if err := t.ctx.Err(); err != nil {
		return true
	}
	return t.state == TimeWindowTaskStateError
}
