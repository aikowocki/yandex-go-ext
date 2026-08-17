package core

import (
	"fmt"
	"time"
)

// Attr содержит пару ключ-значение для structured logging.
type Attr struct {
	Key   string
	Value any
}

// String создаёт строковый атрибут.
func String(key, value string) Attr { return Attr{Key: key, Value: value} }

// Int создаёт целочисленный атрибут.
func Int(key string, value int) Attr { return Attr{Key: key, Value: value} }

// Int32 создаёт атрибут int32.
func Int32(key string, value int32) Attr { return Attr{Key: key, Value: value} }

// Int64 создаёт атрибут int64.
func Int64(key string, value int64) Attr { return Attr{Key: key, Value: value} }

// Bool создаёт логический атрибут.
func Bool(key string, value bool) Attr { return Attr{Key: key, Value: value} }

// Duration создаёт атрибут длительности.
func Duration(key string, value time.Duration) Attr {
	return Attr{Key: key, Value: value}
}

// Any создаёт атрибут произвольного типа.
func Any(key string, value any) Attr { return Attr{Key: key, Value: value} }

// Err создаёт атрибут ошибки.
func Err(err error) Attr { return Attr{Key: "error", Value: err} }

// UUID создаёт строковый атрибут из fmt.Stringer.
func UUID(key string, value fmt.Stringer) Attr {
	if value == nil {
		return Attr{Key: key, Value: nil}
	}
	return String(key, value.String())
}

// NormalizeArgs приводит смешанные аргументы логгера к списку атрибутов.
func NormalizeArgs(args []any) []Attr {
	if len(args) == 0 {
		return nil
	}
	attrs := make([]Attr, 0, len(args)/2)
	for index := 0; index < len(args); index++ {
		if attr, ok := args[index].(Attr); ok {
			attrs = append(attrs, attr)
			continue
		}
		key, ok := args[index].(string)
		if !ok || index+1 >= len(args) {
			attrs = append(attrs, Attr{Key: "!BADKEY", Value: args[index]})
			continue
		}
		attrs = append(attrs, Attr{Key: key, Value: args[index+1]})
		index++
	}
	return attrs
}
