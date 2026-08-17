package testsupport

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
)

// Recorder хранит структурированные события в памяти и при необходимости
// дублирует их в io.Writer, например в testing.TB.Output().
type Recorder struct {
	mu      sync.RWMutex
	events  []logging.Event
	output  io.Writer
	closed  bool
	maxSize int
}

// NewRecorder создаёт recorder для логов теста.
func NewRecorder(output io.Writer) *Recorder {
	return &Recorder{output: output, maxSize: 10_000}
}

// Record сохраняет одно структурированное событие.
func (r *Recorder) Record(level logging.Level, message string, attrs ...logging.Attr) {
	if r == nil {
		return
	}
	event := logging.Event{
		Time:    time.Now().UTC(),
		Level:   level,
		Message: message,
		Fields:  fieldsMap(attrs),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if len(r.events) >= r.maxSize {
		copy(r.events, r.events[1:])
		r.events[len(r.events)-1] = event
	} else {
		r.events = append(r.events, event)
	}
	if r.output != nil {
		data, err := json.Marshal(event)
		if err == nil {
			_, _ = fmt.Fprintln(r.output, string(data))
		}
	}
}

// Records возвращает копию записанных событий.
func (r *Recorder) Records() []logging.Event {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]logging.Event, len(r.events))
	copy(result, r.events)
	for index := range result {
		result[index].Fields = cloneFields(result[index].Fields)
	}
	return result
}

// Clear удаляет все записанные события.
func (r *Recorder) Clear() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.events = nil
	r.mu.Unlock()
}

// Close закрывает recorder и запрещает новые записи.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.output = nil
	r.mu.Unlock()
}

func fieldsMap(attrs []logging.Attr) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	fields := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		fields[attr.Key] = attr.Value
	}
	return fields
}

func cloneFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	clone := make(map[string]any, len(fields))
	for key, value := range fields {
		clone[key] = value
	}
	return clone
}
