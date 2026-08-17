package core

import "encoding/json"

// Level задаёт уровень логирования.
type Level int

const (
	// DebugLevel — подробные отладочные сообщения.
	DebugLevel Level = iota
	// InfoLevel — информационные сообщения.
	InfoLevel
	// WarnLevel — предупреждения.
	WarnLevel
	// ErrorLevel — ошибки.
	ErrorLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	default:
		return "info"
	}
}

// MarshalJSON сериализует уровень как строку.
func (l Level) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}
