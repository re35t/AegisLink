package agent

import (
	"context"
	"errors"
)

var (
	ErrNotFound             = errors.New("agent not found")
	ErrInvalidInstructions  = errors.New("invalid agent instructions")
	ErrInstructionsConflict = errors.New("agent instructions version conflict")
)

type Repository interface {
	List(context.Context, string) ([]Agent, error)
	Default(context.Context, string) (Agent, error)
	Get(context.Context, string, string) (Agent, error)
	GetInstructions(context.Context, string, string) (Instructions, error)
	UpdateInstructions(context.Context, string, string, InstructionsUpdate) (Instructions, error)
}
