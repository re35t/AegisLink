package agentindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPClient struct {
	baseURL           string
	registrationToken string
	queryToken        string
	client            *http.Client
}

func NewHTTPClient(baseURL, registrationToken, queryToken string, timeout time.Duration) (*HTTPClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(registrationToken) == "" || strings.TrimSpace(queryToken) == "" || timeout <= 0 {
		return nil, fmt.Errorf("%w: incomplete Index client configuration", ErrUnavailable)
	}
	return &HTTPClient{
		baseURL: baseURL, registrationToken: strings.TrimSpace(registrationToken), queryToken: strings.TrimSpace(queryToken),
		client: &http.Client{Timeout: timeout},
	}, nil
}

func (client *HTTPClient) Register(ctx context.Context, idempotencyKey string) (string, error) {
	var response struct {
		AgentAddr string `json:"agentAddr"`
	}
	status, err := client.do(ctx, http.MethodPost, "/api/v1/registry/agents", client.registrationToken, idempotencyKey, struct{}{}, &response)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return "", fmt.Errorf("%w: Index registration returned status %d", ErrUnavailable, status)
	}
	if response.AgentAddr == "" {
		return "", fmt.Errorf("%w: Index registration omitted agentAddr", ErrUnavailable)
	}
	return response.AgentAddr, nil
}

func (client *HTTPClient) Publish(ctx context.Context, agentAddr string, snapshot Snapshot) error {
	status, err := client.do(ctx, http.MethodPut, "/api/v1/registry/agents/"+agentAddr+"/representation", client.registrationToken, "", snapshot, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return fmt.Errorf("%w: Index publication returned status %d", ErrUnavailable, status)
	}
	return nil
}

func (client *HTTPClient) Search(ctx context.Context, embedding []float32, topK int) ([]Candidate, error) {
	var response struct {
		Candidates []Candidate `json:"candidates"`
	}
	body := struct {
		SchemaVersion  string    `json:"schemaVersion"`
		EncoderProfile string    `json:"encoderProfile"`
		Embedding      []float32 `json:"embedding"`
		TopK           int       `json:"topK,omitempty"`
	}{QuerySchema, EncoderProfile, embedding, topK}
	status, err := client.do(ctx, http.MethodPost, "/api/v1/discovery/search", client.queryToken, "", body, &response)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: Index search returned status %d", ErrUnavailable, status)
	}
	if response.Candidates == nil {
		response.Candidates = []Candidate{}
	}
	return response.Candidates, nil
}

func (client *HTTPClient) do(ctx context.Context, method, path, token, idempotencyKey string, body, output any) (int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("%w: encode Index request: %v", ErrUnavailable, err)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("%w: create Index request: %v", ErrUnavailable, err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("%w: request Index: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return response.StatusCode, fmt.Errorf("%w: Index status %d: %s", ErrUnavailable, response.StatusCode, strings.TrimSpace(string(message)))
	}
	if output != nil {
		if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(output); err != nil {
			return response.StatusCode, fmt.Errorf("%w: decode Index response: %v", ErrUnavailable, err)
		}
	}
	return response.StatusCode, nil
}
