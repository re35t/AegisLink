package agentindex

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/re35t/AegisLink/internal/agent"
	"golang.org/x/text/unicode/norm"
)

type ProfileReader interface {
	Get(context.Context, string, string) (agent.Profile, error)
}

type Service struct {
	repository Repository
	profiles   ProfileReader
	embedder   Embedder
	client     Client
}

func NewService(repository Repository, profiles ProfileReader, embedder Embedder, client Client) (*Service, error) {
	if repository == nil || profiles == nil || embedder == nil || client == nil {
		return nil, fmt.Errorf("%w: repository, Profile reader, encoder, and Index client are required", ErrUnavailable)
	}
	return &Service{repository: repository, profiles: profiles, embedder: embedder, client: client}, nil
}

func (service *Service) Sync(ctx context.Context, principalID, agentID string) error {
	profile, err := service.profiles.Get(ctx, principalID, agentID)
	if err != nil {
		return err
	}
	state, err := service.repository.Get(ctx, principalID, agentID)
	if err != nil {
		return err
	}
	agentAddr := state.AgentAddr
	if agentAddr == "" {
		agentAddr, err = service.client.Register(ctx, "server-agent-"+agentID)
		if err != nil {
			service.fail(ctx, principalID, agentID, err)
			return err
		}
		if err := service.repository.SaveRegistration(ctx, principalID, agentID, agentAddr); err != nil {
			return err
		}
	}
	units, err := profileUnits(profile)
	if err != nil {
		return err
	}
	texts := make([]string, len(units))
	for index := range units {
		texts[index] = units[index].text
	}
	embeddings, err := service.embedder.Embed(ctx, texts)
	if err != nil {
		service.fail(ctx, principalID, agentID, err)
		return fmt.Errorf("%w: encode Profile: %v", ErrUnavailable, err)
	}
	vectors := make([]FactVector, len(units))
	for index, unit := range units {
		vectors[index] = FactVector{VectorID: unit.vectorID, SourceDigest: digest(unit.text), Embedding: embeddings[index]}
	}
	revision, err := service.repository.ReserveRevision(ctx, principalID, agentID)
	if err != nil {
		return err
	}
	snapshot := Snapshot{
		SchemaVersion: RepresentationSchema, Revision: revision, EncoderProfile: EncoderProfile,
		SourceSetDigest: sourceSetDigest(vectors), Vectors: vectors,
	}
	if err := service.client.Publish(ctx, agentAddr, snapshot); err != nil {
		service.fail(ctx, principalID, agentID, err)
		return err
	}
	return service.repository.MarkPublished(ctx, principalID, agentID, revision)
}

func (service *Service) Search(ctx context.Context, principalID, agentID, query string, topK int) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 4000 || topK < 1 || topK > 50 {
		return nil, ErrInvalid
	}
	if _, err := service.profiles.Get(ctx, principalID, agentID); err != nil {
		return nil, err
	}
	vectors, err := service.embedder.Embed(ctx, []string{norm.NFC.String(query)})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return service.client.Search(ctx, vectors[0], topK)
}

func (service *Service) fail(ctx context.Context, principalID, agentID string, cause error) {
	_ = service.repository.MarkFailed(ctx, principalID, agentID, cause)
}

type profileUnit struct {
	vectorID string
	text     string
}

func profileUnits(profile agent.Profile) ([]profileUnit, error) {
	units := make([]profileUnit, 0, 1+len(profile.Capabilities)+len(profile.ConfirmedFacts))
	if indexable(profile.Identity.Disclosure) {
		value, _ := json.Marshal(map[string]any{"name": profile.Identity.Name, "description": profile.Identity.Description})
		units = append(units, newUnit("identity", profile.Identity.ID, "kind=identity\nvalue="+string(value)))
	}
	for _, capability := range profile.Capabilities {
		if !indexable(capability.Disclosure) {
			continue
		}
		value, _ := json.Marshal(map[string]any{"name": capability.Name, "description": capability.Description, "kind": capability.Kind, "tags": capability.Tags})
		units = append(units, newUnit("capability", capability.ID, "kind=capability\nvalue="+string(value)))
	}
	for _, fact := range profile.ConfirmedFacts {
		if fact.Subject != agent.FactSubjectAgent || !indexable(fact.Disclosure) {
			continue
		}
		value, err := json.Marshal(fact.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: encode confirmed Fact %s", ErrInvalid, fact.ID)
		}
		text := fmt.Sprintf("kind=confirmed-fact\nnamespace=%s\nkey=%s\nvalue=%s", fact.Namespace, fact.Key, value)
		units = append(units, newUnit("fact", fact.ID, text))
	}
	if len(units) > MaximumVectors {
		return nil, fmt.Errorf("%w: Profile exposes more than %d index vectors", ErrInvalid, MaximumVectors)
	}
	sort.Slice(units, func(i, j int) bool { return units[i].vectorID < units[j].vectorID })
	return units, nil
}

func indexable(policy agent.DisclosurePolicy) bool {
	return policy.Indexable && policy.Visibility == agent.VisibilityPublic && policy.Allows(agent.ChannelAgentFacts, "", false)
}

func newUnit(kind, id, text string) profileUnit {
	identity := sha256.Sum256([]byte(kind + "\x00" + id))
	return profileUnit{vectorID: kind + ":" + hex.EncodeToString(identity[:16]), text: norm.NFC.String(text)}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sourceSetDigest(vectors []FactVector) string {
	ordered := append([]FactVector(nil), vectors...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].VectorID < ordered[j].VectorID })
	hasher := sha256.New()
	for _, vector := range ordered {
		writeString(hasher, vector.VectorID)
		writeString(hasher, vector.SourceDigest)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func writeString(hasher hash.Hash, value string) {
	_ = binary.Write(hasher, binary.BigEndian, uint32(len(value)))
	_, _ = hasher.Write([]byte(value))
}
