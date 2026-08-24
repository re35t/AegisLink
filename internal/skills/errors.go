package skills

import "errors"

var (
	ErrInvalid            = errors.New("invalid skill")
	ErrInvalidBundle      = errors.New("invalid skill bundle")
	ErrTooLarge           = errors.New("skill bundle too large")
	ErrResourceUnreadable = errors.New("skill resource is not readable text")
	ErrNotFound           = errors.New("skill not found")
	ErrConflict           = errors.New("skill version label already refers to different content")
)
