package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oklog/ulid/v2"
	"github.com/re35t/AegisLink/internal/mcp"
)

type MCPRepository struct {
	database *sql.DB
}

var _ mcp.Repository = (*MCPRepository)(nil)

func NewMCPRepository(database *sql.DB) *MCPRepository {
	return &MCPRepository{database: database}
}

func (repository *MCPRepository) ListLibrary(ctx context.Context, principalID string) ([]mcp.Server, error) {
	return repository.list(ctx, principalID, "")
}

func (repository *MCPRepository) ListForAgent(ctx context.Context, principalID, agentID string) ([]mcp.Server, error) {
	return repository.list(ctx, principalID, agentID)
}

func (repository *MCPRepository) list(ctx context.Context, principalID, agentID string) ([]mcp.Server, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       bindings.server_id IS NOT NULL, COALESCE(bindings.enabled, false), servers.status,
		       COALESCE(servers.protocol_version, ''), COALESCE(servers.last_error, ''),
		       servers.last_checked_at, servers.created_at, servers.updated_at
		FROM mcp_servers servers
		LEFT JOIN agent_mcp_servers bindings
		  ON bindings.server_id=servers.id AND bindings.agent_id=NULLIF($2, '')
		WHERE servers.owner_principal_id=$1
		ORDER BY servers.name`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	defer rows.Close()
	items := make([]mcp.Server, 0)
	for rows.Next() {
		item, err := scanMCPServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan MCP server: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Tools, err = repository.loadTools(ctx, items[index].ID, agentID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (repository *MCPRepository) Create(ctx context.Context, item mcp.Server, agentID string) (mcp.Server, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("begin MCP server create: %w", err)
	}
	defer transaction.Rollback()
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO mcp_servers (id, owner_principal_id, name, endpoint, transport, status)
		VALUES ($1, $2, $3, $4, 'streamable-http', 'unchecked')
		RETURNING created_at, updated_at`, item.ID, item.OwnerPrincipalID, item.Name, item.Endpoint,
	).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return mcp.Server{}, mcp.ErrConflict
		}
		return mcp.Server{}, fmt.Errorf("create MCP server: %w", err)
	}
	if agentID != "" {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
			VALUES ($1, $2, $3, true)`, item.OwnerPrincipalID, agentID, item.ID); err != nil {
			return mcp.Server{}, fmt.Errorf("bind new MCP server: %w", err)
		}
		item.Bound = true
		item.Enabled = true
	}
	if err := transaction.Commit(); err != nil {
		return mcp.Server{}, fmt.Errorf("commit MCP server create: %w", err)
	}
	item.Transport = "streamable-http"
	item.Status = "unchecked"
	item.Tools = []mcp.Tool{}
	return item, nil
}

func (repository *MCPRepository) Get(ctx context.Context, principalID, serverID string) (mcp.Server, error) {
	item, err := scanMCPServer(repository.database.QueryRowContext(ctx, `
		SELECT id, owner_principal_id, name, endpoint, transport, false, false, status,
		       COALESCE(protocol_version, ''), COALESCE(last_error, ''), last_checked_at, created_at, updated_at
		FROM mcp_servers WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID))
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, fmt.Errorf("get MCP server: %w", err)
	}
	item.Tools, err = repository.loadTools(ctx, item.ID, "")
	return item, err
}

func (repository *MCPRepository) GetForAgent(ctx context.Context, principalID, agentID, serverID string) (mcp.Server, error) {
	item, err := scanMCPServer(repository.database.QueryRowContext(ctx, `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       bindings.server_id IS NOT NULL, COALESCE(bindings.enabled, false), servers.status,
		       COALESCE(servers.protocol_version, ''), COALESCE(servers.last_error, ''),
		       servers.last_checked_at, servers.created_at, servers.updated_at
		FROM mcp_servers servers
		LEFT JOIN agent_mcp_servers bindings ON bindings.server_id=servers.id AND bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND servers.id=$3`, principalID, agentID, serverID))
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, fmt.Errorf("get Agent MCP server: %w", err)
	}
	item.Tools, err = repository.loadTools(ctx, item.ID, agentID)
	return item, err
}

func (repository *MCPRepository) Update(ctx context.Context, principalID, serverID, name, endpoint string) (mcp.Server, error) {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE mcp_servers SET name=$3, endpoint=$4, status='unchecked', protocol_version=NULL,
		       last_error=NULL, last_checked_at=NULL, updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, name, endpoint)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return mcp.Server{}, mcp.ErrConflict
		}
		return mcp.Server{}, fmt.Errorf("update MCP server: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) BindServer(ctx context.Context, principalID, agentID, serverID string) (mcp.Server, error) {
	result, err := repository.database.ExecContext(ctx, `
		INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
		SELECT $1, $2, id, true FROM mcp_servers WHERE id=$3 AND owner_principal_id=$1
		ON CONFLICT (agent_id, server_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, serverID)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("bind MCP server: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.GetForAgent(ctx, principalID, agentID, serverID)
}

func (repository *MCPRepository) UnbindServer(ctx context.Context, principalID, agentID, serverID string) error {
	result, err := repository.database.ExecContext(ctx, `
		DELETE FROM agent_mcp_servers WHERE owner_principal_id=$1 AND agent_id=$2 AND server_id=$3`, principalID, agentID, serverID)
	if err != nil {
		return fmt.Errorf("unbind MCP server: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) Delete(ctx context.Context, principalID, serverID string) error {
	result, err := repository.database.ExecContext(ctx, `DELETE FROM mcp_servers WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID)
	if err != nil {
		return fmt.Errorf("delete MCP server: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) ReplaceTools(ctx context.Context, principalID, serverID string, tools []mcp.Tool, protocol string) (mcp.Server, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("begin MCP refresh: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `
		UPDATE mcp_servers SET status='connected', protocol_version=$3, last_error=NULL,
		       last_checked_at=now(), updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, protocol)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("mark MCP server connected: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO mcp_tools (id, server_id, name, description, input_schema, risk_level)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (server_id, name) DO UPDATE
			SET description=EXCLUDED.description, input_schema=EXCLUDED.input_schema,
			    discovered_at=now(), updated_at=now()`,
			ulid.Make().String(), serverID, tool.Name, tool.Description, tool.InputSchema, tool.RiskLevel,
		); err != nil {
			return mcp.Server{}, fmt.Errorf("upsert MCP tool: %w", err)
		}
	}
	if len(names) == 0 {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=$1`, serverID); err != nil {
			return mcp.Server{}, fmt.Errorf("remove stale MCP tools: %w", err)
		}
	} else if _, err := transaction.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id=$1 AND NOT (name=ANY($2))`, serverID, names); err != nil {
		return mcp.Server{}, fmt.Errorf("remove stale MCP tools: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return mcp.Server{}, fmt.Errorf("commit MCP refresh: %w", err)
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) MarkError(ctx context.Context, principalID, serverID, message string) error {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE mcp_servers SET status='error', last_error=$3, last_checked_at=now(), updated_at=now()
		WHERE owner_principal_id=$1 AND id=$2`, principalID, serverID, message)
	if err != nil {
		return fmt.Errorf("mark MCP server error: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) UpdateToolRisk(ctx context.Context, principalID, serverID, toolID string, risk mcp.RiskLevel) (mcp.Server, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("begin MCP risk update: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `
		UPDATE mcp_tools tools SET risk_level=$4, updated_at=now()
		FROM mcp_servers servers
		WHERE tools.server_id=servers.id AND servers.owner_principal_id=$1
		  AND servers.id=$2 AND tools.id=$3`, principalID, serverID, toolID, risk)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("update MCP tool risk: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	if risk != mcp.ReadOnly {
		if _, err := transaction.ExecContext(ctx, `UPDATE agent_mcp_tools SET enabled=false, updated_at=now() WHERE tool_id=$1`, toolID); err != nil {
			return mcp.Server{}, fmt.Errorf("disable risky MCP tool bindings: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return mcp.Server{}, fmt.Errorf("commit MCP risk update: %w", err)
	}
	return repository.Get(ctx, principalID, serverID)
}

func (repository *MCPRepository) BindTool(ctx context.Context, principalID, agentID, toolID string) (mcp.Tool, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return mcp.Tool{}, fmt.Errorf("begin MCP tool bind: %w", err)
	}
	defer transaction.Rollback()
	var serverID string
	err = transaction.QueryRowContext(ctx, `
		SELECT tools.server_id FROM mcp_tools tools
		JOIN mcp_servers servers ON servers.id=tools.server_id
		WHERE tools.id=$2 AND servers.owner_principal_id=$1`, principalID, toolID).Scan(&serverID)
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Tool{}, fmt.Errorf("find MCP tool server: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO agent_mcp_servers (owner_principal_id, agent_id, server_id, enabled)
		VALUES ($1, $2, $3, true)
		ON CONFLICT (agent_id, server_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, serverID); err != nil {
		return mcp.Tool{}, fmt.Errorf("ensure MCP server binding: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO agent_mcp_tools (owner_principal_id, agent_id, server_id, tool_id, enabled)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (agent_id, tool_id) DO UPDATE SET enabled=true, updated_at=now()`, principalID, agentID, serverID, toolID); err != nil {
		return mcp.Tool{}, fmt.Errorf("bind MCP tool: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return mcp.Tool{}, fmt.Errorf("commit MCP tool bind: %w", err)
	}
	_, tool, err := repository.GetToolForAgent(ctx, principalID, agentID, toolID)
	return tool, err
}

func (repository *MCPRepository) UnbindTool(ctx context.Context, principalID, agentID, toolID string) error {
	result, err := repository.database.ExecContext(ctx, `
		DELETE FROM agent_mcp_tools WHERE owner_principal_id=$1 AND agent_id=$2 AND tool_id=$3`, principalID, agentID, toolID)
	if err != nil {
		return fmt.Errorf("unbind MCP tool: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) GetToolForAgent(ctx context.Context, principalID, agentID, toolID string) (mcp.Server, mcp.Tool, error) {
	var server mcp.Server
	var tool mcp.Tool
	err := repository.database.QueryRowContext(ctx, `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       server_bindings.server_id IS NOT NULL, COALESCE(server_bindings.enabled, false), servers.status,
		       COALESCE(servers.protocol_version, ''), COALESCE(servers.last_error, ''),
		       servers.last_checked_at, servers.created_at, servers.updated_at,
		       tools.id, tools.server_id, tools.name, tools.description, tools.input_schema,
		       COALESCE(tool_bindings.enabled, false), tools.risk_level
		FROM mcp_tools tools
		JOIN mcp_servers servers ON servers.id=tools.server_id
		LEFT JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
		LEFT JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND tools.id=$3`, principalID, agentID, toolID).Scan(
		&server.ID, &server.OwnerPrincipalID, &server.Name, &server.Endpoint, &server.Transport,
		&server.Bound, &server.Enabled, &server.Status, &server.ProtocolVersion, &server.LastError,
		&server.LastCheckedAt, &server.CreatedAt, &server.UpdatedAt,
		&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.InputSchema, &tool.Enabled, &tool.RiskLevel,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, mcp.Tool{}, fmt.Errorf("get Agent MCP tool: %w", err)
	}
	return server, tool, nil
}

func (repository *MCPRepository) RuntimeTools(ctx context.Context, principalID, agentID string) ([]mcp.RuntimeTool, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT tools.id, servers.id, servers.name, tools.name, tools.description, tools.input_schema
		FROM mcp_servers servers
		JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
		JOIN mcp_tools tools ON tools.server_id=servers.id
		JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND server_bindings.enabled AND servers.status='connected'
		  AND tool_bindings.enabled AND tools.risk_level='read-only'
		ORDER BY servers.name, tools.name`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list runtime MCP tools: %w", err)
	}
	defer rows.Close()
	items := make([]mcp.RuntimeTool, 0)
	for rows.Next() {
		var item mcp.RuntimeTool
		if err := rows.Scan(&item.ToolID, &item.ServerID, &item.ServerName, &item.Name, &item.Description, &item.InputSchema); err != nil {
			return nil, fmt.Errorf("scan runtime MCP tool: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *MCPRepository) RuntimeTarget(ctx context.Context, principalID, agentID, serverID, toolName string) (mcp.Server, mcp.Tool, error) {
	var server mcp.Server
	var tool mcp.Tool
	err := repository.database.QueryRowContext(ctx, `
		SELECT servers.id, servers.owner_principal_id, servers.name, servers.endpoint, servers.transport,
		       true, server_bindings.enabled, servers.status, COALESCE(servers.protocol_version, ''),
		       COALESCE(servers.last_error, ''), servers.last_checked_at, servers.created_at, servers.updated_at,
		       tools.id, tools.server_id, tools.name, tools.description, tools.input_schema, tool_bindings.enabled, tools.risk_level
		FROM mcp_servers servers
		JOIN agent_mcp_servers server_bindings ON server_bindings.server_id=servers.id AND server_bindings.agent_id=$2
		JOIN mcp_tools tools ON tools.server_id=servers.id
		JOIN agent_mcp_tools tool_bindings ON tool_bindings.tool_id=tools.id AND tool_bindings.agent_id=$2
		WHERE servers.owner_principal_id=$1 AND servers.id=$3 AND tools.name=$4`, principalID, agentID, serverID, toolName).Scan(
		&server.ID, &server.OwnerPrincipalID, &server.Name, &server.Endpoint, &server.Transport,
		&server.Bound, &server.Enabled, &server.Status, &server.ProtocolVersion, &server.LastError,
		&server.LastCheckedAt, &server.CreatedAt, &server.UpdatedAt,
		&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.InputSchema, &tool.Enabled, &tool.RiskLevel,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, mcp.Tool{}, fmt.Errorf("get runtime MCP target: %w", err)
	}
	return server, tool, nil
}

func (repository *MCPRepository) loadTools(ctx context.Context, serverID, agentID string) ([]mcp.Tool, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT tools.id, tools.server_id, tools.name, tools.description, tools.input_schema,
		       COALESCE(bindings.enabled, false), tools.risk_level
		FROM mcp_tools tools
		LEFT JOIN agent_mcp_tools bindings ON bindings.tool_id=tools.id AND bindings.agent_id=NULLIF($2, '')
		WHERE tools.server_id=$1 ORDER BY tools.name`, serverID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}
	defer rows.Close()
	items := make([]mcp.Tool, 0)
	for rows.Next() {
		var item mcp.Tool
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Name, &item.Description, &item.InputSchema, &item.Enabled, &item.RiskLevel); err != nil {
			return nil, fmt.Errorf("scan MCP tool: %w", err)
		}
		if !json.Valid(item.InputSchema) {
			return nil, errors.New("stored MCP tool schema is invalid JSON")
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanMCPServer(row scanner) (mcp.Server, error) {
	var item mcp.Server
	err := row.Scan(
		&item.ID, &item.OwnerPrincipalID, &item.Name, &item.Endpoint, &item.Transport,
		&item.Bound, &item.Enabled, &item.Status, &item.ProtocolVersion, &item.LastError,
		&item.LastCheckedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func requireMCPChanged(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read MCP affected rows: %w", err)
	}
	if changed == 0 {
		return mcp.ErrNotFound
	}
	return nil
}
