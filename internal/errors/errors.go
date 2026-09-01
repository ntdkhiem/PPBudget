package errors

import "errors"

var (
	ErrNotFound     = errors.New("resource not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrDuplicate    = errors.New("duplicate transaction")
	ErrConflict     = errors.New("conflict")
)
