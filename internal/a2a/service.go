package a2a

import (
	"context"

	a2asdk "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/re35t/AegisLink/internal/agent"
)

type ProfileReader interface {
	Get(context.Context, string, string) (agent.Profile, error)
}

type Readiness struct {
	Publishable bool     `json:"publishable"`
	Blockers    []string `json:"blockers"`
}

type Preview struct {
	Draft     a2asdk.AgentCard `json:"draft"`
	Readiness Readiness        `json:"readiness"`
}

type Service struct {
	profiles ProfileReader
}

func NewService(profiles ProfileReader) *Service {
	return &Service{profiles: profiles}
}

func (service *Service) Preview(ctx context.Context, principalID, agentID string) (Preview, error) {
	profile, err := service.profiles.Get(ctx, principalID, agentID)
	if err != nil {
		return Preview{}, err
	}
	skills := make([]a2asdk.AgentSkill, 0)
	for _, capability := range profile.Capabilities {
		if capability.Kind != agent.CapabilitySkill || !capability.Callable || !capability.Disclosure.Allows(agent.ChannelAgentCard, "", false) {
			continue
		}
		skills = append(skills, a2asdk.AgentSkill{
			ID: capability.ID, Name: capability.Name, Description: capability.Description,
			Tags: append([]string{}, capability.Tags...), InputModes: []string{"text/plain"}, OutputModes: []string{"text/plain"},
		})
	}
	draft := a2asdk.AgentCard{
		SupportedInterfaces: []*a2asdk.AgentInterface{}, Capabilities: a2asdk.AgentCapabilities{},
		DefaultInputModes: []string{"text/plain"}, DefaultOutputModes: []string{"text/plain"},
		Name: profile.Identity.Name, Description: profile.Identity.Description, IconURL: profile.Identity.AvatarURL,
		Version: "draft-" + formatVersion(profile.Version), Skills: skills,
		SecuritySchemes: a2asdk.NamedSecuritySchemes{}, SecurityRequirements: a2asdk.SecurityRequirementsOptions{},
	}
	return Preview{Draft: draft, Readiness: Readiness{Publishable: false, Blockers: []string{"general-a2a-publication-disabled"}}}, nil
}

func formatVersion(version int64) string {
	if version < 1 {
		return "1"
	}
	result := ""
	for version > 0 {
		result = string(rune('0'+version%10)) + result
		version /= 10
	}
	return result
}
