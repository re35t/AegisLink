package mcp

import "errors"

var (
	ErrInvalid     = errors.New("invalid MCP configuration")
	ErrNotFound    = errors.New("MCP resource not found")
	ErrConflict    = errors.New("MCP server already exists")
	ErrConnection  = errors.New("MCP connection failed")
	ErrForbidden   = errors.New("MCP tool requires approval")
	ErrUnavailable = errors.New("MCP tool is unavailable")
)
