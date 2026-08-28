package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var ErrEmbedding = errors.New("embedding provider failed")

type EmbeddingConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
	Timeout    time.Duration
}

// Embedder is deliberately domain-neutral; callers own the meaning of input text.
type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}

type OpenAICompatibleEmbedder struct {
	endpoint   string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

func NewOpenAICompatibleEmbedder(config EmbeddingConfig) (*OpenAICompatibleEmbedder, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || strings.TrimSpace(config.Model) == "" || config.Dimensions < 1 || config.Timeout <= 0 {
		return nil, fmt.Errorf("%w: invalid embedding configuration", ErrEmbedding)
	}
	return &OpenAICompatibleEmbedder{
		endpoint: baseURL + "/embeddings", apiKey: strings.TrimSpace(config.APIKey),
		model: strings.TrimSpace(config.Model), dimensions: config.Dimensions,
		client: &http.Client{Timeout: config.Timeout},
	}, nil
}

func (embedder *OpenAICompatibleEmbedder) Embed(ctx context.Context, input []string) ([][]float32, error) {
	if len(input) == 0 {
		return [][]float32{}, nil
	}
	payload, err := json.Marshal(struct {
		Model      string   `json:"model"`
		Input      []string `json:"input"`
		Dimensions int      `json:"dimensions,omitempty"`
	}{Model: embedder.model, Input: input, Dimensions: embedder.dimensions})
	if err != nil {
		return nil, fmt.Errorf("%w: encode request: %v", ErrEmbedding, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, embedder.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrEmbedding, err)
	}
	request.Header.Set("Content-Type", "application/json")
	if embedder.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+embedder.apiKey)
	}
	response, err := embedder.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request: %v", ErrEmbedding, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("%w: status %d: %s", ErrEmbedding, response.StatusCode, strings.TrimSpace(string(body)))
	}
	var body struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrEmbedding, err)
	}
	if len(body.Data) != len(input) {
		return nil, fmt.Errorf("%w: expected %d vectors, received %d", ErrEmbedding, len(input), len(body.Data))
	}
	sort.Slice(body.Data, func(i, j int) bool { return body.Data[i].Index < body.Data[j].Index })
	result := make([][]float32, len(body.Data))
	for index, item := range body.Data {
		if item.Index != index || len(item.Embedding) != embedder.dimensions || !finiteNonzero(item.Embedding) {
			return nil, fmt.Errorf("%w: vector %d is missing, malformed, or zero", ErrEmbedding, index)
		}
		result[index] = item.Embedding
	}
	return result, nil
}

func finiteNonzero(vector []float32) bool {
	nonzero := false
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
		nonzero = nonzero || value != 0
	}
	return nonzero
}
