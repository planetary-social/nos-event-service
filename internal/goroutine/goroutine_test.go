package goroutine_test

import (
	"strings"
	"testing"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/goroutine"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_PropagatesError(t *testing.T) {
	errCh := make(chan error, 1)
	expectedErr := errors.New("test error")

	goroutine.Run(logging.NewDevNullLogger(), errCh, func() error {
		return expectedErr
	})

	err := <-errCh
	assert.ErrorIs(t, err, expectedErr)
}

func TestRun_RecoversPanic(t *testing.T) {
	errCh := make(chan error, 1)

	goroutine.Run(logging.NewDevNullLogger(), errCh, func() error {
		panic("something went wrong")
	})

	err := <-errCh
	require.Error(t, err)
	assert.Contains(t, err.Error(), "goroutine panicked: something went wrong")
	assert.Contains(t, err.Error(), "goroutine.go")
}

func TestRun_RecoversPanicWithError(t *testing.T) {
	errCh := make(chan error, 1)

	goroutine.Run(logging.NewDevNullLogger(), errCh, func() error {
		panic(errors.New("panic error"))
	})

	err := <-errCh
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "panic error"))
}

func TestRun_SuccessfulCompletion(t *testing.T) {
	errCh := make(chan error, 1)

	goroutine.Run(logging.NewDevNullLogger(), errCh, func() error {
		return nil
	})

	err := <-errCh
	assert.NoError(t, err)
}
