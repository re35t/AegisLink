package agent

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("agent not found")

type Repository interface {
	List(context.Context, string) ([]Agent, error)
	Default(context.Context, string) (Agent, error)
	Get(context.Context, string, string) (Agent, error)
}
