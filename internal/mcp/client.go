package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maximumTools      = 500
	maximumResultSize = 256 * 1024
)

type OfficialClient struct {
	timeout      time.Duration
	allowPrivate bool
}

func NewOfficialClient(timeout time.Duration, allowPrivate bool) *OfficialClient {
	return &OfficialClient{timeout: timeout, allowPrivate: allowPrivate}
}

func (client *OfficialClient) ValidateEndpoint(endpoint string) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ErrInvalid
	}
	if parsed.Scheme != "https" && !(client.allowPrivate && parsed.Scheme == "http") {
		return ErrInvalid
	}
	return nil
}

func (client *OfficialClient) Discover(ctx context.Context, endpoint string) ([]Tool, string, error) {
	session, err := client.connect(ctx, endpoint)
	if err != nil {
		return nil, "", err
	}
	defer session.Close()
	tools := make([]Tool, 0)
	cursor := ""
	for {
		result, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, "", fmt.Errorf("list MCP tools: %w", err)
		}
		for _, discovered := range result.Tools {
			if discovered == nil || discovered.Name == "" || len(tools) >= maximumTools {
				continue
			}
			inputSchema, err := json.Marshal(discovered.InputSchema)
			if err != nil {
				return nil, "", fmt.Errorf("encode MCP tool schema: %w", err)
			}
			risk := ExternalWrite
			if discovered.Annotations != nil && discovered.Annotations.ReadOnlyHint {
				risk = ReadOnly
			} else if discovered.Annotations != nil && discovered.Annotations.DestructiveHint != nil && *discovered.Annotations.DestructiveHint {
				risk = Destructive
			}
			tools = append(tools, Tool{
				Name: discovered.Name, Description: discovered.Description, InputSchema: inputSchema,
				Enabled: false, RiskLevel: risk,
			})
		}
		if result.NextCursor == "" || len(tools) >= maximumTools {
			break
		}
		cursor = result.NextCursor
	}
	protocol := ""
	if initialized := session.InitializeResult(); initialized != nil {
		protocol = initialized.ProtocolVersion
	}
	return tools, protocol, nil
}

func (client *OfficialClient) Invoke(ctx context.Context, endpoint, toolName, arguments string) (string, error) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(arguments), &decoded); err != nil {
		return "", fmt.Errorf("decode MCP tool arguments: %w", err)
	}
	session, err := client.connect(ctx, endpoint)
	if err != nil {
		return "", err
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: toolName, Arguments: decoded})
	if err != nil {
		return "", fmt.Errorf("call MCP tool: %w", err)
	}
	parts := make([]string, 0, len(result.Content)+1)
	for _, content := range result.Content {
		switch typed := content.(type) {
		case *mcpsdk.TextContent:
			parts = append(parts, typed.Text)
		default:
			encoded, marshalErr := content.MarshalJSON()
			if marshalErr != nil {
				return "", fmt.Errorf("encode MCP result content: %w", marshalErr)
			}
			parts = append(parts, string(encoded))
		}
	}
	if result.StructuredContent != nil {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", fmt.Errorf("encode structured MCP result: %w", err)
		}
		parts = append(parts, string(encoded))
	}
	output := strings.Join(parts, "\n")
	if len(output) > maximumResultSize {
		return "", errors.New("MCP tool result exceeds size limit")
	}
	if result.IsError {
		return "", fmt.Errorf("MCP tool returned an error: %s", output)
	}
	return output, nil
}

func (client *OfficialClient) connect(ctx context.Context, endpoint string) (*mcpsdk.ClientSession, error) {
	if err := client.ValidateEndpoint(endpoint); err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout: client.timeout,
		Transport: &http.Transport{
			DialContext:       client.safeDialContext,
			ForceAttemptHTTP2: true,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("MCP redirects are disabled")
		},
	}
	transport := &mcpsdk.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: httpClient, MaxRetries: -1, DisableStandaloneSSE: true,
	}
	sdkClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "aegislink", Version: "0.3.0"}, &mcpsdk.ClientOptions{Capabilities: &mcpsdk.ClientCapabilities{}})
	session, err := sdkClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect MCP server: %w", err)
	}
	return session, nil
}

func (client *OfficialClient) safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP host: %w", err)
	}
	for _, resolved := range addresses {
		if !client.allowPrivate && unsafeIP(resolved.IP) {
			continue
		}
		dialer := net.Dialer{Timeout: client.timeout}
		return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
	}
	return nil, errors.New("MCP endpoint resolves only to blocked network addresses")
}

func unsafeIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
