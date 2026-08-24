package mcp

import "context"

type Repository interface {
	List(context.Context, string, string) ([]Server, error)
	Create(context.Context, Server) (Server, error)
	Get(context.Context, string, string, string) (Server, error)
	SetEnabled(context.Context, string, string, string, bool) (Server, error)
	Delete(context.Context, string, string, string) error
	ReplaceTools(context.Context, string, string, string, []Tool, string) (Server, error)
	MarkError(context.Context, string, string, string, string) error
	UpdateTool(context.Context, string, string, string, string, bool, RiskLevel) (Server, error)
	RuntimeTools(context.Context, string, string) ([]RuntimeTool, error)
	RuntimeTarget(context.Context, string, string, string, string) (Server, Tool, error)
}

type Client interface {
	ValidateEndpoint(string) error
	Discover(context.Context, string) ([]Tool, string, error)
	Invoke(context.Context, string, string, string) (string, error)
}
