package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleEmbedderSendsPinnedDimensionsAndOrdersResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" || request.Header.Get("Authorization") != "Bearer encoder-secret" {
			t.Errorf("unexpected encoder request: path=%q authorization=%q", request.URL.Path, request.Header.Get("Authorization"))
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct {
			Model      string   `json:"model"`
			Input      []string `json:"input"`
			Dimensions int      `json:"dimensions"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Model != "discovery-encoder" || body.Dimensions != 3 || len(body.Input) != 2 {
			t.Errorf("unexpected encoder body: %#v", body)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[{"index":1,"embedding":[0,1,0]},{"index":0,"embedding":[1,0,0]}]}`))
	}))
	defer server.Close()

	embedder, err := NewOpenAICompatibleEmbedder(EmbeddingConfig{
		BaseURL: server.URL + "/v1", APIKey: "encoder-secret", Model: "discovery-encoder",
		Dimensions: 3, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := embedder.Embed(t.Context(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("encoder response was not restored to input order: %#v", vectors)
	}
}

func TestOpenAICompatibleEmbedderRejectsMalformedVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[{"index":0,"embedding":[0,0,0]}]}`))
	}))
	defer server.Close()

	embedder, err := NewOpenAICompatibleEmbedder(EmbeddingConfig{
		BaseURL: server.URL, Model: "discovery-encoder", Dimensions: 3, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.Embed(t.Context(), []string{"query"}); err == nil {
		t.Fatal("expected the zero vector to be rejected")
	}
}
