package messaging

// SafeValue нормализует пустое значение атрибута.
func SafeValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
