package agentindex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientImplementsRegisterPublishAndVectorSearchProtocol(t *testing.T) {
	requests := make(chan string, 3)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests <- request.Method + " " + request.URL.Path
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/registry/agents":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer registration-secret" || request.Header.Get("Idempotency-Key") != "server-agent-agent-1" {
				t.Errorf("unexpected registration request: method=%s headers=%v", request.Method, request.Header)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			response.WriteHeader(http.StatusCreated)
			_, _ = response.Write([]byte(`{"agentAddr":"agent_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`))
		case "/api/v1/registry/agents/agent_01ARZ3NDEKTSV4RRFFQ69G5FAV/representation":
			if request.Method != http.MethodPut || request.Header.Get("Authorization") != "Bearer registration-secret" {
				t.Errorf("unexpected publication request: method=%s headers=%v", request.Method, request.Header)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			var snapshot Snapshot
			if err := json.NewDecoder(request.Body).Decode(&snapshot); err != nil {
				t.Error(err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			if snapshot.Revision != 1 || snapshot.EncoderProfile != EncoderProfile || len(snapshot.Vectors) != 1 {
				t.Errorf("unexpected snapshot: %#v", snapshot)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		case "/api/v1/discovery/search":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer query-secret" {
				t.Errorf("unexpected search request: method=%s headers=%v", request.Method, request.Header)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			var body struct {
				SchemaVersion  string    `json:"schemaVersion"`
				EncoderProfile string    `json:"encoderProfile"`
				Embedding      []float32 `json:"embedding"`
				TopK           int       `json:"topK"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Error(err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			if body.SchemaVersion != QuerySchema || body.EncoderProfile != EncoderProfile || body.TopK != 5 || len(body.Embedding) != EmbeddingDimensions {
				t.Errorf("unexpected search body: schema=%q profile=%q topK=%d dimensions=%d", body.SchemaVersion, body.EncoderProfile, body.TopK, len(body.Embedding))
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = response.Write([]byte(`{"candidates":[{"agentAddr":"agent_candidate","score":0.8,"matchedVectorId":"fact:one","representationRevision":2}]}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, "registration-secret", "query-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	address, err := client.Register(t.Context(), "server-agent-agent-1")
	if err != nil || address != "agent_01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("registration address=%q error=%v", address, err)
	}
	embedding := make([]float32, EmbeddingDimensions)
	embedding[0] = 1
	if err := client.Publish(t.Context(), address, Snapshot{
		SchemaVersion: RepresentationSchema, Revision: 1, EncoderProfile: EncoderProfile,
		SourceSetDigest: "sha256:test", Vectors: []FactVector{{VectorID: "fact:one", SourceDigest: "sha256:test", Embedding: embedding}},
	}); err != nil {
		t.Fatal(err)
	}
	candidates, err := client.Search(t.Context(), embedding, 5)
	if err != nil || len(candidates) != 1 || candidates[0].AgentAddr != "agent_candidate" {
		t.Fatalf("search candidates=%#v error=%v", candidates, err)
	}
	close(requests)
	if len(requests) != 3 {
		t.Fatalf("Index request count = %d", len(requests))
	}
}
