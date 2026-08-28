package domain

import "errors"

var (
	// ErrNotFound означает, что сущность не найдена.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists означает, что сущность уже существует.
	ErrAlreadyExists = errors.New("already exists")
	// ErrConflict означает конфликт состояния сущности.
	ErrConflict = errors.New("conflict")
	// ErrInvalidInput означает некорректные входные данные.
	ErrInvalidInput = errors.New("invalid input")
	// ErrFileTooLarge означает превышение допустимого размера файла.
	ErrFileTooLarge = errors.New("file too large")
	// ErrInvalidFormat означает неподдерживаемый формат данных.
	ErrInvalidFormat = errors.New("invalid file format")
	// ErrForbidden означает отсутствие прав на операцию.
	ErrForbidden = errors.New("forbidden")
	// ErrProcessingFailed означает ошибку обработки.
	ErrProcessingFailed = errors.New("processing failed")
	// ErrStorageError означает ошибку объектного хранилища.
	ErrStorageError = errors.New("storage error")
)
