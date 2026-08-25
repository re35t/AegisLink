package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/mcp"
	"gorm.io/gorm"
)

type MCPRepository struct {
	database *gorm.DB
}

var _ mcp.Repository = (*MCPRepository)(nil)

func NewMCPRepository(database *gorm.DB) *MCPRepository {
	return &MCPRepository{database: database}
}

func (repository *MCPRepository) ListLibrary(ctx context.Context, principalID string) ([]mcp.Server, error) {
	return repository.list(ctx, principalID, "")
}

func (repository *MCPRepository) ListForAgent(ctx context.Context, principalID, agentID string) ([]mcp.Server, error) {
	return repository.list(ctx, principalID, agentID)
}

func (repository *MCPRepository) list(ctx context.Context, principalID, agentID string) ([]mcp.Server, error) {
	items := make([]mcp.Server, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       bindings.server_id IS NOT NULL AS bound, COALESCE(bindings.enabled, false) AS enabled, servers.status,
		       COALESCE(servers.protocol_version, '') AS protocol_version,
		       COALESCE(servers.last_error, '') AS last_error,
		       servers.last_checked_at, servers.created_at, servers.updated_at
		FROM mcp_servers servers
		LEFT JOIN agent_mcp_servers bindings
		  ON bindings.server_id=servers.id AND bindings.agent_id=NULLIF($2, '')
		WHERE servers.owner_principal_id=$1
		ORDER BY servers.name`, principalID, agentID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list MCP servers: %w", result.Error)
	}
	for index := range items {
		items[index].Tools, result.Error = repository.loadTools(repository.database.WithContext(ctx), items[index].ID, agentID)
		if result.Error != nil {
			return nil, result.Error
		}
	}
	return items, nil
}

func (repository *MCPRepository) Create(ctx context.Context, item mcp.Server, agentID string) (mcp.Server, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var timestamps struct {
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		err := scanOne(transaction, &timestamps, `
			INSERT INTO mcp_servers (id, owner_principal_id, name, endpoint, transport, status)
			VALUES ($1, $2, $3, $4, 'streamable-http', 'unchecked')
			RETURNING created_at, updated_at`, item.ID, item.OwnerPrincipalID, item.Name, item.Endpoint)
		if isUniqueViolation(err) {
			return mcp.ErrConflict
		}
		if err != nil {
			return fmt.Errorf("create MCP server: %w", err)
		}
		item.CreatedAt = timestamps.CreatedAt
		item.UpdatedAt = timestamps.UpdatedAt
		if agentID != "" {
			result := exec(transaction, `
				INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
				VALUES ($1, $2, $3, true)`, item.OwnerPrincipalID, agentID, item.ID)
			if result.Error != nil {
				return fmt.Errorf("bind new MCP server: %w", result.Error)
			}
			item.Bound = true
			item.Enabled = true
		}
		return nil
	})
	if err != nil {
		return mcp.Server{}, err
	}
	item.Transport = "streamable-http"
	item.Status = "unchecked"
	item.Tools = []mcp.Tool{}
	return item, nil
}

func (repository *MCPRepository) Get(ctx context.Context, principalID, serverID string) (mcp.Server, error) {
	var item mcp.Server
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT id, owner_principal_id, name, endpoint, transport, false AS bound, false AS enabled, status,
		       COALESCE(protocol_version, '') AS protocol_version, COALESCE(last_error, '') AS last_error,
		       last_checked_at, created_at, updated_at
		FROM mcp_servers WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mcp.Server{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, fmt.Errorf("get MCP server: %w", err)
	}
	item.Tools, err = repository.loadTools(repository.database.WithContext(ctx), item.ID, "")
	return item, err
}

func (repository *MCPRepository) GetForAgent(ctx context.Context, principalID, agentID, serverID string) (mcp.Server, error) {
	var item mcp.Server
	err := scanOne(repository.database.WithContext(ctx), &item, `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       bindings.server_id IS NOT NULL AS bound, COALESCE(bindings.enabled, false) AS enabled, servers.status,
		       COALESCE(servers.protocol_version, '') AS protocol_version,
		       COALESCE(servers.last_error, '') AS last_error,
		       servers.last_checked_at, servers.created_at, servers.updated_at
		FROM mcp_servers servers
		LEFT JOIN agent_mcp_servers bindings ON bindings.server_id=servers.id AND bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND servers.id=$3`, principalID, agentID, serverID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mcp.Server{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, fmt.Errorf("get Agent MCP server: %w", err)
	}
	item.Tools, err = repository.loadTools(repository.database.WithContext(ctx), item.ID, agentID)
	return item, err
}

func (repository *MCPRepository) Update(ctx context.Context, principalID, serverID, name, endpoint string) (mcp.Server, error) {
	result := exec(repository.database.WithContext(ctx), `
		UPDATE mcp_servers SET name=$3, endpoint=$4, status='unchecked', protocol_version=NULL,
		       last_error=NULL, last_checked_at=NULL, updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, name, endpoint)
	if isUniqueViolation(result.Error) {
		return mcp.Server{}, mcp.ErrConflict
	}
	if result.Error != nil {
		return mcp.Server{}, fmt.Errorf("update MCP server: %w", result.Error)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) BindServer(ctx context.Context, principalID, agentID, serverID string) (mcp.Server, error) {
	result := exec(repository.database.WithContext(ctx), `
		INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
		SELECT $1, $2, id, true FROM mcp_servers WHERE id=$3 AND owner_principal_id=$1
		ON CONFLICT (agent_id, server_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, serverID)
	if result.Error != nil {
		return mcp.Server{}, fmt.Errorf("bind MCP server: %w", result.Error)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.GetForAgent(ctx, principalID, agentID, serverID)
}

func (repository *MCPRepository) UnbindServer(ctx context.Context, principalID, agentID, serverID string) error {
	result := exec(repository.database.WithContext(ctx), `
		DELETE FROM agent_mcp_servers WHERE owner_principal_id=$1 AND agent_id=$2 AND server_id=$3`, principalID, agentID, serverID)
	if result.Error != nil {
		return fmt.Errorf("unbind MCP server: %w", result.Error)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) Delete(ctx context.Context, principalID, serverID string) error {
	result := exec(repository.database.WithContext(ctx), `DELETE FROM mcp_servers WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID)
	if result.Error != nil {
		return fmt.Errorf("delete MCP server: %w", result.Error)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) ReplaceTools(ctx context.Context, principalID, serverID string, tools []mcp.Tool, protocol string) (mcp.Server, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := exec(transaction, `
			UPDATE mcp_servers SET status='connected', protocol_version=$3, last_error=NULL,
			       last_checked_at=now(), updated_at=now()
			WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, protocol)
		if result.Error != nil {
			return fmt.Errorf("mark MCP server connected: %w", result.Error)
		}
		if err := requireMCPChanged(result); err != nil {
			return err
		}
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, tool.Name)
			result = exec(transaction, `
				INSERT INTO mcp_tools (id, server_id, name, description, input_schema, risk_level)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (server_id, name) DO UPDATE
				SET description=EXCLUDED.description, input_schema=EXCLUDED.input_schema,
				    discovered_at=now(), updated_at=now()`,
				ulid.Make().String(), serverID, tool.Name, tool.Description, tool.InputSchema, tool.RiskLevel)
			if result.Error != nil {
				return fmt.Errorf("upsert MCP tool: %w", result.Error)
			}
		}
		if len(names) == 0 {
			result = exec(transaction, `DELETE FROM mcp_tools WHERE server_id=$1`, serverID)
		} else {
			result = transaction.Exec(`DELETE FROM mcp_tools WHERE server_id = ? AND name NOT IN ?`, serverID, names)
		}
		if result.Error != nil {
			return fmt.Errorf("remove stale MCP tools: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) MarkError(ctx context.Context, principalID, serverID, message string) error {
	result := exec(repository.database.WithContext(ctx), `
		UPDATE mcp_servers SET status='error', last_error=$3, last_checked_at=now(), updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, message)
	if result.Error != nil {
		return fmt.Errorf("mark MCP server error: %w", result.Error)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) UpdateToolRisk(ctx context.Context, principalID, serverID, toolID string, risk mcp.RiskLevel) (mcp.Server, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := exec(transaction, `
			UPDATE mcp_tools tools SET risk_level=$4, updated_at=now()
			FROM mcp_servers servers
			WHERE tools.server_id=servers.id AND servers.owner_principal_id=$1
			  AND servers.id=$2 AND tools.id=$3`, principalID, serverID, toolID, risk)
		if result.Error != nil {
			return fmt.Errorf("update MCP tool risk: %w", result.Error)
		}
		if err := requireMCPChanged(result); err != nil {
			return err
		}
		if risk != mcp.ReadOnly {
			result = exec(transaction, `UPDATE agent_mcp_tools SET enabled=false, updated_at=now() WHERE tool_id=$1`, toolID)
			if result.Error != nil {
				return fmt.Errorf("disable risky MCP tool bindings: %w", result.Error)
			}
		}
		return nil
	})
	if err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) BindTool(ctx context.Context, principalID, agentID, toolID string) (mcp.Tool, error) {
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var row struct{ ServerID string }
		err := scanOne(transaction, &row, `
			SELECT tools.server_id FROM mcp_tools tools
			JOIN mcp_servers servers ON servers.id=tools.server_id
			WHERE tools.id=$2 AND servers.owner_principal_id=$1`, principalID, toolID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return mcp.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find MCP tool server: %w", err)
		}
		result := exec(transaction, `
			INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (agent_id, server_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, row.ServerID)
		if result.Error != nil {
			return fmt.Errorf("ensure MCP server binding: %w", result.Error)
		}
		result = exec(transaction, `
			INSERT INTO agent_mcp_tools (owner_principal_id, agent_id, server_id, tool_id, enabled)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (agent_id, tool_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, row.ServerID, toolID)
		if result.Error != nil {
			return fmt.Errorf("bind MCP tool: %w", result.Error)
		}
		return nil
	})
	if err != nil {
		return mcp.Tool{}, err
	}
	_, tool, err := repository.GetToolForAgent(ctx, principalID, agentID, toolID)
	return tool, err
}

func (repository *MCPRepository) UnbindTool(ctx context.Context, principalID, agentID, toolID string) error {
	result := exec(repository.database.WithContext(ctx), `
		DELETE FROM agent_mcp_tools WHERE owner_principal_id=$1 AND agent_id=$2 AND tool_id=$3`, principalID, agentID, toolID)
	if result.Error != nil {
		return fmt.Errorf("unbind MCP tool: %w", result.Error)
	}
	return requireMCPChanged(result)
}

type mcpServerToolRow struct {
	ServerID         string `gorm:"column:server_id"`
	OwnerPrincipalID string `gorm:"column:owner_principal_id"`
	ServerName       string `gorm:"column:server_name"`
	Endpoint         string
	Transport        string
	Bound            bool
	ServerEnabled    bool `gorm:"column:server_enabled"`
	Status           string
	ProtocolVersion  string
	LastError        string
	LastCheckedAt    *time.Time
	ServerCreatedAt  time.Time `gorm:"column:server_created_at"`
	ServerUpdatedAt  time.Time `gorm:"column:server_updated_at"`
	ToolID           string    `gorm:"column:tool_id"`
	ToolServerID     string    `gorm:"column:tool_server_id"`
	ToolName         string    `gorm:"column:tool_name"`
	ToolDescription  string    `gorm:"column:tool_description"`
	InputSchema      json.RawMessage
	ToolEnabled      bool `gorm:"column:tool_enabled"`
	RiskLevel        mcp.RiskLevel
}

func (row mcpServerToolRow) values() (mcp.Server, mcp.Tool) {
	server := mcp.Server{
		ID: row.ServerID, OwnerPrincipalID: row.OwnerPrincipalID, Name: row.ServerName,
		Endpoint: row.Endpoint, Transport: row.Transport, Bound: row.Bound, Enabled: row.ServerEnabled,
		Status: row.Status, ProtocolVersion: row.ProtocolVersion, LastError: row.LastError,
		LastCheckedAt: row.LastCheckedAt, CreatedAt: row.ServerCreatedAt, UpdatedAt: row.ServerUpdatedAt,
	}
	tool := mcp.Tool{
		ID: row.ToolID, ServerID: row.ToolServerID, Name: row.ToolName, Description: row.ToolDescription,
		InputSchema: row.InputSchema, Enabled: row.ToolEnabled, RiskLevel: row.RiskLevel,
	}
	return server, tool
}

func (repository *MCPRepository) GetToolForAgent(ctx context.Context, principalID, agentID, toolID string) (mcp.Server, mcp.Tool, error) {
	var row mcpServerToolRow
	err := scanOne(repository.database.WithContext(ctx), &row, mcpServerToolQuery(`servers.owner_principal_id=$1 AND tools.id=$3`), principalID, agentID, toolID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mcp.Server{}, mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, mcp.Tool{}, fmt.Errorf("get Agent MCP tool: %w", err)
	}
	server, tool := row.values()
	return server, tool, nil
}

func (repository *MCPRepository) RuntimeTools(ctx context.Context, principalID, agentID string) ([]mcp.RuntimeTool, error) {
	items := make([]mcp.RuntimeTool, 0)
	result := raw(repository.database.WithContext(ctx), `
		SELECT tools.id AS tool_id, servers.id AS server_id, servers.name AS server_name,
		       tools.name, tools.description, tools.input_schema
		FROM mcp_servers servers
		JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
		JOIN mcp_tools tools ON tools.server_id=servers.id
		JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND server_bindings.enabled AND servers.status='connected'
		  AND tool_bindings.enabled AND tools.risk_level='read-only'
		ORDER BY servers.name, tools.name`, principalID, agentID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list runtime MCP tools: %w", result.Error)
	}
	return items, nil
}

func (repository *MCPRepository) RuntimeTarget(ctx context.Context, principalID, agentID, serverID, toolName string) (mcp.Server, mcp.Tool, error) {
	var row mcpServerToolRow
	err := scanOne(repository.database.WithContext(ctx), &row, mcpRuntimeToolQuery, principalID, agentID, serverID, toolName)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mcp.Server{}, mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, mcp.Tool{}, fmt.Errorf("get runtime MCP target: %w", err)
	}
	server, tool := row.values()
	return server, tool, nil
}

func (repository *MCPRepository) loadTools(database *gorm.DB, serverID, agentID string) ([]mcp.Tool, error) {
	items := make([]mcp.Tool, 0)
	result := raw(database, `
		SELECT tools.id, tools.server_id, tools.name, tools.description, tools.input_schema,
		       COALESCE(bindings.enabled, false) AS enabled, tools.risk_level
		FROM mcp_tools tools
		LEFT JOIN agent_mcp_tools bindings ON bindings.tool_id=tools.id AND bindings.agent_id=NULLIF($2, '')
		WHERE tools.server_id=$1 ORDER BY tools.name`, serverID, agentID).Scan(&items)
	if result.Error != nil {
		return nil, fmt.Errorf("list MCP tools: %w", result.Error)
	}
	for _, item := range items {
		if !json.Valid(item.InputSchema) {
			return nil, errors.New("stored MCP tool schema is invalid JSON")
		}
	}
	return items, nil
}

func requireMCPChanged(result *gorm.DB) error {
	if result.RowsAffected == 0 {
		return mcp.ErrNotFound
	}
	return nil
}

func mcpServerToolQuery(condition string) string {
	return `
		SELECT servers.id AS server_id, servers.owner_principal_id,
		       servers.name AS server_name, servers.endpoint, servers.transport,
		       server_bindings.server_id IS NOT NULL AS bound,
		       COALESCE(server_bindings.enabled, false) AS server_enabled, servers.status,
		       COALESCE(servers.protocol_version, '') AS protocol_version,
		       COALESCE(servers.last_error, '') AS last_error,
		       servers.last_checked_at, servers.created_at AS server_created_at, servers.updated_at AS server_updated_at,
		       tools.id AS tool_id, tools.server_id AS tool_server_id, tools.name AS tool_name,
		       tools.description AS tool_description, tools.input_schema,
		       COALESCE(tool_bindings.enabled, false) AS tool_enabled, tools.risk_level
		FROM mcp_tools tools
		JOIN mcp_servers servers ON servers.id=tools.server_id
		LEFT JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
		LEFT JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
		WHERE ` + condition
}

const mcpRuntimeToolQuery = `
	SELECT servers.id AS server_id, servers.owner_principal_id,
	       servers.name AS server_name, servers.endpoint, servers.transport,
	       true AS bound, server_bindings.enabled AS server_enabled, servers.status,
	       COALESCE(servers.protocol_version, '') AS protocol_version,
	       COALESCE(servers.last_error, '') AS last_error,
	       servers.last_checked_at, servers.created_at AS server_created_at, servers.updated_at AS server_updated_at,
	       tools.id AS tool_id, tools.server_id AS tool_server_id, tools.name AS tool_name,
	       tools.description AS tool_description, tools.input_schema,
	       tool_bindings.enabled AS tool_enabled, tools.risk_level
	FROM mcp_servers servers
	JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
	JOIN mcp_tools tools ON tools.server_id=servers.id
	JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
	WHERE servers.owner_principal_id=$1 AND servers.id=$3 AND tools.name=$4`
