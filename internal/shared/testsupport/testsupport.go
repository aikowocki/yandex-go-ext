package testsupport

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

var recorders sync.Map // map[testing.TB]*Recorder

// Context создаёт recorder и добавляет его в контекст теста.
func Context(tb testing.TB) context.Context {
	tb.Helper()
	recorder := NewRecorder(tb.Output())
	recorders.Store(tb, recorder)
	tb.Cleanup(func() {
		recorders.Delete(tb)
		recorder.Close()
	})
	return logging.WithSink(tb.Context(), recorder)
}

// Assert возвращает fluent-проверки для recorder текущего теста.
func Assert(tb testing.TB) *Assertions {
	tb.Helper()
	value, ok := recorders.Load(tb)
	if !ok {
		tb.Fatalf("logging test context is not initialized; call testsupport.Context(t) first")
	}
	return &Assertions{tb: tb, recorder: value.(*Recorder)}
}

// Assertions содержит проверки записей structured logging.
type Assertions struct {
	tb       testing.TB
	recorder *Recorder
}

// Contains проверяет наличие сообщения.
func (a *Assertions) Contains(message string) *Assertions {
	a.contains(message)
	return a
}

// CountMessage проверяет количество одинаковых сообщений.
func (a *Assertions) CountMessage(message string, expected int) *Assertions {
	a.countMessage(message, expected)
	return a
}

// HasField проверяет значение поля сообщения.
func (a *Assertions) HasField(key string, expected any) *Assertions {
	a.hasField(key, expected)
	return a
}

// HasLevel проверяет наличие записи с указанным уровнем.
func (a *Assertions) HasLevel(level logging.Level) *Assertions {
	a.hasLevel(level)
	return a
}

// NotContains проверяет отсутствие сообщения.
func (a *Assertions) NotContains(message string) *Assertions {
	a.notContains(message)
	return a
}

// NoErrors проверяет отсутствие error-level записей.
func (a *Assertions) NoErrors() *Assertions {
	a.noErrors()
	return a
}

// Last возвращает последнюю запись, если recorder не пуст.
func (a *Assertions) Last() (logging.Event, bool) {
	a.tb.Helper()
	records := a.recorder.Records()
	if len(records) == 0 {
		return logging.Event{}, false
	}
	return records[len(records)-1], true
}

func (a *Assertions) contains(message string) bool {
	a.tb.Helper()
	for _, event := range a.recorder.Records() {
		if event.Message == message {
			return true
		}
	}
	a.tb.Errorf("expected log message %q", message)
	return false
}

func (a *Assertions) countMessage(message string, expected int) bool {
	a.tb.Helper()
	actual := 0
	for _, event := range a.recorder.Records() {
		if event.Message == message {
			actual++
		}
	}
	if actual != expected {
		a.tb.Errorf("expected log message %q %d time(s), got %d", message, expected, actual)
		return false
	}
	return true
}

func (a *Assertions) hasField(key string, expected any) bool {
	a.tb.Helper()
	for _, event := range a.recorder.Records() {
		if value, ok := event.Fields[key]; ok && reflect.DeepEqual(value, expected) {
			return true
		}
	}
	a.tb.Errorf("expected log field %q to equal %#v", key, expected)
	return false
}

func (a *Assertions) hasLevel(level logging.Level) bool {
	a.tb.Helper()
	for _, event := range a.recorder.Records() {
		if event.Level == level {
			return true
		}
	}
	a.tb.Errorf("expected log level %q", level)
	return false
}

func (a *Assertions) notContains(message string) bool {
	a.tb.Helper()
	for _, event := range a.recorder.Records() {
		if event.Message == message {
			a.tb.Errorf("did not expect log message %q", message)
			return false
		}
	}
	return true
}

func (a *Assertions) noErrors() bool {
	a.tb.Helper()
	for _, event := range a.recorder.Records() {
		if event.Level == logging.ErrorLevel {
			a.tb.Errorf("did not expect error log %q", event.Message)
			return false
		}
	}
	return true
}

// Contains проверяет наличие сообщения в recorder текущего теста.
func Contains(tb testing.TB, message string) bool {
	return Assert(tb).contains(message)
}

// CountMessage проверяет количество сообщений в recorder текущего теста.
func CountMessage(tb testing.TB, message string, expected int) bool {
	return Assert(tb).countMessage(message, expected)
}

// HasField проверяет поле сообщения в recorder текущего теста.
func HasField(tb testing.TB, key string, expected any) bool {
	return Assert(tb).hasField(key, expected)
}

// HasLevel проверяет уровень сообщения в recorder текущего теста.
func HasLevel(tb testing.TB, level logging.Level) bool {
	return Assert(tb).hasLevel(level)
}

// NotContains проверяет отсутствие сообщения в recorder текущего теста.
func NotContains(tb testing.TB, message string) bool {
	return Assert(tb).notContains(message)
}

// NoErrors проверяет отсутствие ошибок в recorder текущего теста.
func NoErrors(tb testing.TB) bool {
	return Assert(tb).noErrors()
}
