package skills

import "context"

type Repository interface {
	List(context.Context, string, string) ([]Skill, error)
	Install(context.Context, Skill) (Skill, error)
	SetEnabled(context.Context, string, string, string, bool) (Skill, error)
	Uninstall(context.Context, string, string, string) error
	Enabled(context.Context, string, string) ([]Skill, error)
}
