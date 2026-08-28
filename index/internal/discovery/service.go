package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/re35t/AegisLink/index/internal/registry"
)

var vectorIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type Service struct {
	store  RepresentationStore
	search VectorSearch
	now    func() time.Time
}

func NewService(store RepresentationStore, search VectorSearch) (*Service, error) {
	if store == nil || search == nil {
		return nil, fmt.Errorf("%w: representation store and vector search are required", ErrInvalidSnapshot)
	}
	return &Service{
		store: store, search: search,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (service *Service) Publish(ctx context.Context, address registry.AgentAddr, snapshot Snapshot) error {
	if !registry.ValidAgentAddr(address) {
		return ErrAgentNotFound
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	return service.store.Replace(ctx, Representation{
		AgentAddr: address, Revision: snapshot.Revision,
		EncoderProfile: snapshot.EncoderProfile, SourceSetDigest: snapshot.SourceSetDigest,
		Vectors: snapshot.Vectors, RequestDigest: requestDigest(snapshot),
		PublishedAt: service.now().UTC().Truncate(time.Microsecond),
	})
}

func (service *Service) Search(ctx context.Context, query Query) (Result, error) {
	topK, err := validateQuery(query)
	if err != nil {
		return Result{}, err
	}
	candidates, err := service.search.Search(ctx, query.EncoderProfile, query.Embedding, topK)
	if err != nil {
		return Result{}, err
	}
	if candidates == nil {
		candidates = []Candidate{}
	}
	return Result{
		SchemaVersion: ResultSchemaVersion, EncoderProfile: query.EncoderProfile,
		Candidates: candidates,
	}, nil
}

// SourceSetDigest defines the stable publisher/Index digest contract: sort by
// vectorId, then hash each length-prefixed vectorId and sourceDigest pair.
func SourceSetDigest(vectors []FactVector) string {
	ordered := slices.Clone(vectors)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].VectorID < ordered[j].VectorID })
	h := sha256.New()
	for _, vector := range ordered {
		writeString(h, vector.VectorID)
		writeString(h, vector.SourceDigest)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func validateSnapshot(snapshot Snapshot) error {
	if snapshot.SchemaVersion != RepresentationSchemaVersion {
		return fmt.Errorf("%w: schemaVersion must be %q", ErrInvalidSnapshot, RepresentationSchemaVersion)
	}
	if snapshot.EncoderProfile != EncoderProfile {
		return ErrUnsupportedProfile
	}
	if snapshot.Revision < 1 {
		return fmt.Errorf("%w: revision must be positive", ErrInvalidSnapshot)
	}
	if len(snapshot.Vectors) > MaxVectorsPerAgent {
		return fmt.Errorf("%w: vectors must contain at most %d items", ErrInvalidSnapshot, MaxVectorsPerAgent)
	}
	seen := make(map[string]struct{}, len(snapshot.Vectors))
	for _, vector := range snapshot.Vectors {
		if !vectorIDPattern.MatchString(vector.VectorID) {
			return fmt.Errorf("%w: invalid vectorId %q", ErrInvalidSnapshot, vector.VectorID)
		}
		if _, duplicate := seen[vector.VectorID]; duplicate {
			return fmt.Errorf("%w: duplicate vectorId %q", ErrInvalidSnapshot, vector.VectorID)
		}
		seen[vector.VectorID] = struct{}{}
		if !validDigest(vector.SourceDigest) {
			return fmt.Errorf("%w: sourceDigest must be a lowercase SHA-256 digest", ErrInvalidSnapshot)
		}
		if err := validateEmbedding(vector.Embedding); err != nil {
			return fmt.Errorf("%w: vector %q: %v", ErrInvalidSnapshot, vector.VectorID, err)
		}
	}
	if !validDigest(snapshot.SourceSetDigest) || snapshot.SourceSetDigest != SourceSetDigest(snapshot.Vectors) {
		return fmt.Errorf("%w: sourceSetDigest does not match the vector source set", ErrInvalidSnapshot)
	}
	return nil
}

func validateQuery(query Query) (int, error) {
	if query.SchemaVersion != QuerySchemaVersion {
		return 0, fmt.Errorf("%w: schemaVersion must be %q", ErrInvalidQuery, QuerySchemaVersion)
	}
	if query.EncoderProfile != EncoderProfile {
		return 0, ErrUnsupportedProfile
	}
	if err := validateEmbedding(query.Embedding); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}
	topK := query.TopK
	if topK == 0 {
		topK = DefaultTopK
	}
	if topK < 1 || topK > MaxTopK {
		return 0, fmt.Errorf("%w: topK must be between 1 and %d", ErrInvalidQuery, MaxTopK)
	}
	return topK, nil
}

func validateEmbedding(embedding []float32) error {
	if len(embedding) != EmbeddingDimensions {
		return fmt.Errorf("embedding must contain exactly %d values", EmbeddingDimensions)
	}
	nonzero := false
	for _, value := range embedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding values must be finite float32 numbers")
		}
		nonzero = nonzero || value != 0
	}
	if !nonzero {
		return fmt.Errorf("embedding must not be the zero vector")
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || value[:len("sha256:")] != "sha256:" {
		return false
	}
	decoded, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil && hex.EncodeToString(decoded) == value[len("sha256:"):]
}

func requestDigest(snapshot Snapshot) []byte {
	h := sha256.New()
	writeString(h, snapshot.SchemaVersion)
	writeString(h, snapshot.EncoderProfile)
	writeString(h, snapshot.SourceSetDigest)
	_ = binary.Write(h, binary.BigEndian, snapshot.Revision)
	ordered := slices.Clone(snapshot.Vectors)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].VectorID < ordered[j].VectorID })
	for _, vector := range ordered {
		writeString(h, vector.VectorID)
		writeString(h, vector.SourceDigest)
		for _, value := range vector.Embedding {
			_ = binary.Write(h, binary.BigEndian, math.Float32bits(value))
		}
	}
	return h.Sum(nil)
}

func writeString(h hash.Hash, value string) {
	_ = binary.Write(h, binary.BigEndian, uint32(len(value)))
	_, _ = h.Write([]byte(value))
}
