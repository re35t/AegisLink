package skills

import "errors"

var (
	ErrInvalid  = errors.New("invalid skill")
	ErrNotFound = errors.New("skill not found")
	ErrConflict = errors.New("skill version label already refers to different content")
)
