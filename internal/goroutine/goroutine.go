package goroutine

import (
	"fmt"
	"runtime/debug"

	"github.com/planetary-social/nos-event-service/internal/logging"
)

// Run starts a goroutine that executes fn and sends its return value to errCh.
// If fn panics, the panic is recovered, logged with a full stack trace, and
// converted to an error sent to errCh. This prevents goroutines from silently
// dying and leaving the service in a zombie state.
func Run(logger logging.Logger, errCh chan<- error, fn func() error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				err := fmt.Errorf("goroutine panicked: %v\n%s", r, stack)
				logger.Error().WithError(err).Message("recovered from panic in goroutine")
				errCh <- err
			}
		}()

		errCh <- fn()
	}()
}
