package memory

import "context"

type Repository interface {
	List(context.Context, string, string) ([]Memory, error)
	Create(context.Context, Memory) (Memory, error)
	Update(context.Context, string, string, string, Update) (Memory, error)
	Forget(context.Context, string, string, string) error
	Context(context.Context, string, string, int) ([]Memory, error)
}
