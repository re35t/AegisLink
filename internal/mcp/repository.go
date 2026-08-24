package mcp

import "context"

type Repository interface {
	ListLibrary(context.Context, string) ([]Server, error)
	ListForAgent(context.Context, string, string) ([]Server, error)
	Create(context.Context, Server, string) (Server, error)
	Get(context.Context, string, string) (Server, error)
	GetForAgent(context.Context, string, string, string) (Server, error)
	Update(context.Context, string, string, string, string) (Server, error)
	BindServer(context.Context, string, string, string) (Server, error)
	UnbindServer(context.Context, string, string, string) error
	Delete(context.Context, string, string) error
	ReplaceTools(context.Context, string, string, []Tool, string) (Server, error)
	MarkError(context.Context, string, string, string) error
	UpdateToolRisk(context.Context, string, string, string, RiskLevel) (Server, error)
	BindTool(context.Context, string, string, string) (Tool, error)
	UnbindTool(context.Context, string, string, string) error
	GetToolForAgent(context.Context, string, string, string) (Server, Tool, error)
	RuntimeTools(context.Context, string, string) ([]RuntimeTool, error)
	RuntimeTarget(context.Context, string, string, string, string) (Server, Tool, error)
}

type Client interface {
	ValidateEndpoint(string) error
	Discover(context.Context, string) ([]Tool, string, error)
	Invoke(context.Context, string, string, string) (string, error)
}
