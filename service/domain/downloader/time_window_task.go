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
	filter domain.Filter
	window TimeWindow

	ctx    context.Context
	cancel context.CancelFunc
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
	ctx, cancel := context.WithCancel(ctx)

	t := &TimeWindowTask{
		ctx:    ctx,
		cancel: cancel,
		state:  TimeWindowTaskStateStarted,
		window: window,
	}
	if err := t.updateFilter(kinds, tags, authors); err != nil {
		return nil, errors.New("error updating the filter")
	}
	return t, nil
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

	if t.state != TimeWindowTaskStateDone {
		t.state = TimeWindowTaskStateError
	}
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

func (t *TimeWindowTask) MaybeReset(ctx context.Context, kinds []domain.EventKind, tags []domain.FilterTag, authors []domain.PublicKey) (bool, error) {
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
	t.state = TimeWindowTaskStateStarted

	if err := t.updateFilter(kinds, tags, authors); err != nil {
		return false, errors.New("error updating the filter")
	}

	return true, nil
}

func (t *TimeWindowTask) updateFilter(kinds []domain.EventKind, tags []domain.FilterTag, authors []domain.PublicKey) error {
	filter, err := domain.NewFilter(
		nil,
		kinds,
		tags,
		authors,
		internal.Pointer(t.window.Start()),
		internal.Pointer(t.window.End()),
	)
	if err != nil {
		return errors.Wrap(err, "error creating a filter")
	}

	t.filter = filter

	return nil
}

func (t *TimeWindowTask) isDead() bool {
	if err := t.ctx.Err(); err != nil {
		return true
	}
	return t.state == TimeWindowTaskStateError
}
