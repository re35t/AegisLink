package conversation

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrActiveRun           = errors.New("conversation already has an active run")
	ErrRunNotActive        = errors.New("run is not active")
	ErrInvalidMessage      = errors.New("message is invalid")
	ErrForcedToolNotCalled = errors.New("forced tool was not called")
	ErrForcedToolMismatch  = errors.New("model called a different tool than the forced tool")
)
