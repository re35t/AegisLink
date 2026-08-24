package memory

import "errors"

var (
	ErrInvalid  = errors.New("invalid memory")
	ErrNotFound = errors.New("memory not found")
)
