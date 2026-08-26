package a2a

import (
	"context"
	"testing"

	"github.com/re35t/AegisLink/internal/agent"
)

func TestPreviewMapsDisclosedSkillsButRemainsBlockedWithoutEndpoint(t *testing.T) {
	service := NewService(profileStub{})
	preview, err := service.Preview(t.Context(), "owner", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Readiness.Publishable || len(preview.Readiness.Blockers) != 1 || preview.Readiness.Blockers[0] != "a2a-endpoint-missing" {
		t.Fatalf("readiness = %#v", preview.Readiness)
	}
	if len(preview.Draft.SupportedInterfaces) != 0 || len(preview.Draft.Skills) != 1 || preview.Draft.Skills[0].ID != "skill:review" {
		t.Fatalf("draft = %#v", preview.Draft)
	}
}

type profileStub struct{}

func (profileStub) Get(context.Context, string, string) (agent.Profile, error) {
	policy := agent.DisclosurePolicy{Visibility: agent.VisibilityPublic, Channels: []agent.DisclosureChannel{agent.ChannelAgentCard}}
	return agent.Profile{Version: 2, Identity: agent.ProfileIdentity{Name: "Aegis", Description: "Personal Agent"}, Capabilities: []agent.ProfileCapability{{ID: "skill:review", Name: "Review", Kind: agent.CapabilitySkill, Callable: true, Disclosure: policy}, {ID: "model:hidden", Name: "Hidden", Kind: agent.CapabilityModel, Callable: true, Disclosure: agent.DefaultDisclosurePolicy()}}}, nil
}
