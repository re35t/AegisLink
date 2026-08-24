package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/re35t/AegisLink/internal/mcp"
)

type MCPRepository struct {
	database *sql.DB
}

var _ mcp.Repository = (*MCPRepository)(nil)

func NewMCPRepository(database *sql.DB) *MCPRepository {
	return &MCPRepository{database: database}
}

func (repository *MCPRepository) List(ctx context.Context, principalID, agentID string) ([]mcp.Server, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, owner_principal_id, agent_id, name, endpoint, transport, enabled, status,
		       COALESCE(protocol_version, ''), COALESCE(last_error, ''), last_checked_at, created_at, updated_at
		FROM mcp_servers
		WHERE owner_principal_id=$1 AND agent_id=$2
		ORDER BY name`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	items := make([]mcp.Server, 0)
	for rows.Next() {
		item, err := scanMCPServer(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan MCP server: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Tools, err = repository.loadTools(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (repository *MCPRepository) Create(ctx context.Context, item mcp.Server) (mcp.Server, error) {
	err := repository.database.QueryRowContext(ctx, `
		INSERT INTO mcp_servers (id, owner_principal_id, agent_id, name, endpoint, transport, enabled, status)
		VALUES ($1, $2, $3, $4, $5, 'streamable-http', true, 'unchecked')
		RETURNING created_at, updated_at`, item.ID, item.OwnerPrincipalID, item.AgentID, item.Name, item.Endpoint,
	).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return mcp.Server{}, mcp.ErrConflict
		}
		return mcp.Server{}, fmt.Errorf("create MCP server: %w", err)
	}
	item.Transport = "streamable-http"
	item.Enabled = true
	item.Status = "unchecked"
	item.Tools = []mcp.Tool{}
	return item, nil
}

func (repository *MCPRepository) Get(ctx context.Context, principalID, agentID, serverID string) (mcp.Server, error) {
	item, err := scanMCPServer(repository.database.QueryRowContext(ctx, `
		SELECT id, owner_principal_id, agent_id, name, endpoint, transport, enabled, status,
		       COALESCE(protocol_version, ''), COALESCE(last_error, ''), last_checked_at, created_at, updated_at
		FROM mcp_servers
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3`, principalID, agentID, serverID))
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, fmt.Errorf("get MCP server: %w", err)
	}
	item.Tools, err = repository.loadTools(ctx, item.ID)
	if err != nil {
		return mcp.Server{}, err
	}
	return item, nil
}

func (repository *MCPRepository) SetEnabled(ctx context.Context, principalID, agentID, serverID string, enabled bool) (mcp.Server, error) {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE mcp_servers SET enabled=$4, updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3`, principalID, agentID, serverID, enabled)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("set MCP server enabled: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, agentID, serverID)
}

func (repository *MCPRepository) Delete(ctx context.Context, principalID, agentID, serverID string) error {
	result, err := repository.database.ExecContext(ctx, `
		DELETE FROM mcp_servers
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3`, principalID, agentID, serverID)
	if err != nil {
		return fmt.Errorf("delete MCP server: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) ReplaceTools(ctx context.Context, principalID, agentID, serverID string, tools []mcp.Tool, protocol string) (mcp.Server, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("begin MCP refresh: %w", err)
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `
		UPDATE mcp_servers
		SET status='connected', protocol_version=$4, last_error=NULL, last_checked_at=now(), updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3`, principalID, agentID, serverID, protocol)
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
			INSERT INTO mcp_tools (server_id, name, description, input_schema, enabled, risk_level)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (server_id, name) DO UPDATE
			SET description=EXCLUDED.description, input_schema=EXCLUDED.input_schema,
			    discovered_at=now(), updated_at=now()`,
			serverID, tool.Name, tool.Description, tool.InputSchema, tool.Enabled, tool.RiskLevel,
		); err != nil {
			return mcp.Server{}, fmt.Errorf("upsert MCP tool: %w", err)
		}
	}
	if _, err := transaction.ExecContext(ctx, `
		DELETE FROM mcp_tools WHERE server_id=$1 AND NOT (name=ANY($2))`, serverID, names); err != nil {
		return mcp.Server{}, fmt.Errorf("remove stale MCP tools: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return mcp.Server{}, fmt.Errorf("commit MCP refresh: %w", err)
	}
	return repository.Get(ctx, principalID, agentID, serverID)
}

func (repository *MCPRepository) MarkError(ctx context.Context, principalID, agentID, serverID, message string) error {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE mcp_servers
		SET status='error', last_error=$4, last_checked_at=now(), updated_at=now()
		WHERE owner_principal_id=$1 AND agent_id=$2 AND id=$3`, principalID, agentID, serverID, message)
	if err != nil {
		return fmt.Errorf("mark MCP server error: %w", err)
	}
	return requireMCPChanged(result)
}

func (repository *MCPRepository) UpdateTool(ctx context.Context, principalID, agentID, serverID, toolName string, enabled bool, risk mcp.RiskLevel) (mcp.Server, error) {
	result, err := repository.database.ExecContext(ctx, `
		UPDATE mcp_tools tools
		SET enabled=$5, risk_level=$6, updated_at=now()
		FROM mcp_servers servers
		WHERE tools.server_id=servers.id AND servers.owner_principal_id=$1 AND servers.agent_id=$2
		  AND servers.id=$3 AND tools.name=$4`, principalID, agentID, serverID, toolName, enabled, risk)
	if err != nil {
		return mcp.Server{}, fmt.Errorf("update MCP tool: %w", err)
	}
	if err := requireMCPChanged(result); err != nil {
		return mcp.Server{}, err
	}
	return repository.Get(ctx, principalID, agentID, serverID)
}

func (repository *MCPRepository) RuntimeTools(ctx context.Context, principalID, agentID string) ([]mcp.RuntimeTool, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT servers.id, servers.name, tools.name, tools.description, tools.input_schema
		FROM mcp_servers servers
		JOIN mcp_tools tools ON tools.server_id=servers.id
		WHERE servers.owner_principal_id=$1 AND servers.agent_id=$2
		  AND servers.enabled AND servers.status='connected'
		  AND tools.enabled AND tools.risk_level='read-only'
		ORDER BY servers.name, tools.name`, principalID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list runtime MCP tools: %w", err)
	}
	defer rows.Close()
	items := make([]mcp.RuntimeTool, 0)
	for rows.Next() {
		var item mcp.RuntimeTool
		if err := rows.Scan(&item.ServerID, &item.ServerName, &item.Name, &item.Description, &item.InputSchema); err != nil {
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
		SELECT servers.id, servers.owner_principal_id, servers.agent_id, servers.name, servers.endpoint,
		       servers.transport, servers.enabled, servers.status, COALESCE(servers.protocol_version, ''),
		       COALESCE(servers.last_error, ''), servers.last_checked_at, servers.created_at, servers.updated_at,
		       tools.name, tools.description, tools.input_schema, tools.enabled, tools.risk_level
		FROM mcp_servers servers
		JOIN mcp_tools tools ON tools.server_id=servers.id
		WHERE servers.owner_principal_id=$1 AND servers.agent_id=$2 AND servers.id=$3 AND tools.name=$4`,
		principalID, agentID, serverID, toolName,
	).Scan(
		&server.ID, &server.OwnerPrincipalID, &server.AgentID, &server.Name, &server.Endpoint,
		&server.Transport, &server.Enabled, &server.Status, &server.ProtocolVersion, &server.LastError,
		&server.LastCheckedAt, &server.CreatedAt, &server.UpdatedAt,
		&tool.Name, &tool.Description, &tool.InputSchema, &tool.Enabled, &tool.RiskLevel,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return mcp.Server{}, mcp.Tool{}, mcp.ErrNotFound
	}
	if err != nil {
		return mcp.Server{}, mcp.Tool{}, fmt.Errorf("get runtime MCP target: %w", err)
	}
	return server, tool, nil
}

func (repository *MCPRepository) loadTools(ctx context.Context, serverID string) ([]mcp.Tool, error) {
	rows, err := repository.database.QueryContext(ctx, `
		SELECT name, description, input_schema, enabled, risk_level
		FROM mcp_tools WHERE server_id=$1 ORDER BY name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list MCP tools: %w", err)
	}
	defer rows.Close()
	items := make([]mcp.Tool, 0)
	for rows.Next() {
		var item mcp.Tool
		if err := rows.Scan(&item.Name, &item.Description, &item.InputSchema, &item.Enabled, &item.RiskLevel); err != nil {
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
		&item.ID, &item.OwnerPrincipalID, &item.AgentID, &item.Name, &item.Endpoint, &item.Transport,
		&item.Enabled, &item.Status, &item.ProtocolVersion, &item.LastError, &item.LastCheckedAt,
		&item.CreatedAt, &item.UpdatedAt,
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
