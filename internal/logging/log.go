package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
)

const (
	loggerFieldName  = "name"
	loggerFieldError = "error"
)

type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelError
	LevelDisabled
)

type Logger interface {
	New(name string) Logger
	WithError(err error) Logger
	WithField(key string, v any) Logger

	Error() Entry
	Debug() Entry
	Trace() Entry
}

type Entry interface {
	WithError(err error) Entry
	WithField(key string, v any) Entry
	Message(msg string)
}

type LoggingSystem interface {
	EnabledLevel() Level
	Error() LoggingSystemEntry
	Debug() LoggingSystemEntry
	Trace() LoggingSystemEntry
}

type LoggingSystemEntry interface {
	WithField(key string, v any) LoggingSystemEntry
	Message(msg string)
}

type SystemLogger struct {
	fields map[string]any
	logger LoggingSystem
}

func NewSystemLogger(logger LoggingSystem, name string) Logger {
	if logger.EnabledLevel() >= LevelDisabled {
		return NewDevNullLogger()
	}
	return newSystemLogger(logger, map[string]any{loggerFieldName: name})
}

func newSystemLogger(logger LoggingSystem, fields map[string]any) SystemLogger {
	newLogger := SystemLogger{
		fields: make(map[string]any),

		logger: logger,
	}

	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l SystemLogger) Error() Entry {
	if l.logger.EnabledLevel() > LevelError {
		return newDevNullLoggerEntry()
	}
	return l.withFields(newEntry(l.logger.Error()))
}

func (l SystemLogger) Debug() Entry {
	if l.logger.EnabledLevel() > LevelDebug {
		return newDevNullLoggerEntry()
	}
	return l.withFields(newEntry(l.logger.Debug()))
}

func (l SystemLogger) Trace() Entry {
	if l.logger.EnabledLevel() > LevelTrace {
		return newDevNullLoggerEntry()
	}
	return l.withFields(newEntry(l.logger.Trace()))
}

func (l SystemLogger) New(name string) Logger {
	newLogger := newSystemLogger(l.logger, l.fields)
	v, okExists := l.fields[loggerFieldName]
	if okExists {
		if stringV, okType := v.(string); okType {
			newLogger.fields[loggerFieldName] = stringV + "." + name
			return newLogger
		}
		return newLogger
	}
	newLogger.fields[loggerFieldName] = name
	return newLogger
}

func (l SystemLogger) WithError(err error) Logger {
	newLogger := newSystemLogger(l.logger, l.fields)
	newLogger.fields[loggerFieldError] = err
	return newLogger
}

func (l SystemLogger) WithField(key string, v any) Logger {
	newLogger := newSystemLogger(l.logger, l.fields)
	newLogger.fields[key] = v
	return newLogger
}

func (l SystemLogger) withFields(entry Entry) Entry {
	for k, v := range l.fields {
		entry = entry.WithField(k, v)
	}
	return entry
}

type entry struct {
	loggingSystemEntry LoggingSystemEntry
}

func newEntry(loggingSystemEntry LoggingSystemEntry) entry {
	return entry{loggingSystemEntry: loggingSystemEntry}
}

func (e entry) WithError(err error) Entry {
	return newEntry(e.loggingSystemEntry.WithField(loggerFieldError, err))
}

func (e entry) WithField(key string, v any) Entry {
	return newEntry(e.loggingSystemEntry.WithField(key, v))
}

func (e entry) Message(msg string) {
	e.loggingSystemEntry.Message(msg)
}

type DevNullLogger struct {
}

func NewDevNullLogger() DevNullLogger {
	return DevNullLogger{}
}

func (d DevNullLogger) New(name string) Logger {
	return d
}

func (d DevNullLogger) WithError(err error) Logger {
	return d
}

func (d DevNullLogger) WithField(key string, v any) Logger {
	return d
}

func (d DevNullLogger) Error() Entry {
	return newDevNullLoggerEntry()
}

func (d DevNullLogger) Debug() Entry {
	return newDevNullLoggerEntry()
}

func (d DevNullLogger) Trace() Entry {
	return newDevNullLoggerEntry()
}

type devNullLoggerEntry struct {
}

func newDevNullLoggerEntry() devNullLoggerEntry {
	return devNullLoggerEntry{}
}

func (d devNullLoggerEntry) WithError(err error) Entry {
	return d
}

func (d devNullLoggerEntry) WithField(key string, v any) Entry {
	return d
}

func (d devNullLoggerEntry) Message(msg string) {
}

// ConfigureTimeThrottler returns a throttled version of the provided action function.
// The returned function will execute the action at most once every specified duration.
// TODO: Maybe this should be moved to a separate package.
func ConfigureTimeThrottler(duration time.Duration) func(action func()) {
	var (
		lastExecution time.Time
		mu            sync.Mutex
	)
	return func(action func()) {
		mu.Lock()
		defer mu.Unlock()

		if time.Since(lastExecution) < duration {
			return
		}

		lastExecution = time.Now()
		action()
	}
}

func Inspect(args ...interface{}) {
	for _, arg := range args {
		val := reflect.ValueOf(arg)

		if val.Kind() == reflect.String {
			fmt.Println(arg)
			continue
		}
		// Use String() or MarshalJSON if available
		if val.CanInterface() {
			if marshaler, ok := arg.(json.Marshaler); ok {
				if jsonBytes, err := marshaler.MarshalJSON(); err == nil {
					fmt.Println(string(jsonBytes))
					continue
				}
			}
			if stringer, ok := arg.(fmt.Stringer); ok {
				fmt.Println(stringer.String())
				continue
			}
		}

		spew.Dump(arg)
	}
	println()
}

// Debugging function for development to use voice messages to detect issues
// in the code.
var Say func(string)

func init() {
	if os.Getenv("EVENTS_ENVIRONMENT") != "PRODUCTION" && runtime.GOOS == "darwin" {
		Say = func(text string) {
			cmd := exec.Command("say", text)
			if err := cmd.Run(); err != nil {
				fmt.Println("Failed to execute say command:", err)
			}
		}
	} else {
		Say = func(text string) {
			// This function intentionally left blank.
		}
	}
}

type nopLogger struct{}

func NewNopLogger() Logger {
	return &nopLogger{}
}

func (n *nopLogger) Trace() Entry {
	return &nopEntry{}
}

func (n *nopLogger) Debug() Entry {
	return &nopEntry{}
}

func (n *nopLogger) Info() Entry {
	return &nopEntry{}
}

func (n *nopLogger) Error() Entry {
	return &nopEntry{}
}

func (n *nopLogger) WithError(err error) Logger {
	return n
}

func (n *nopLogger) WithField(key string, value interface{}) Logger {
	return n
}

func (n *nopLogger) New(component string) Logger {
	return n
}

type nopEntry struct{}

func (n *nopEntry) WithError(err error) Entry {
	return n
}

func (n *nopEntry) WithField(key string, value interface{}) Entry {
	return n
}

func (n *nopEntry) Message(msg string) {
	// Do nothing
}
