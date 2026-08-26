package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceNormalizesAndUpdatesInstructions(t *testing.T) {
	repository := &agentRepositoryStub{}
	service := NewService(repository)

	updated, err := service.UpdateInstructions(t.Context(), "owner-1", "agent-1", InstructionsUpdate{
		ExpectedVersion: 1,
		SystemPrompt:    "\n  Answer directly.  \n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.update.SystemPrompt != "Answer directly." || updated.Version != 2 {
		t.Fatalf("unexpected update: stored=%#v result=%#v", repository.update, updated)
	}

	_, err = service.UpdateInstructions(t.Context(), "owner-1", "agent-1", InstructionsUpdate{SystemPrompt: "invalid version"})
	if !errors.Is(err, ErrInvalidInstructions) {
		t.Fatalf("invalid version error = %v", err)
	}
	_, err = service.UpdateInstructions(t.Context(), "owner-1", "agent-1", InstructionsUpdate{
		ExpectedVersion: 2,
		SystemPrompt:    strings.Repeat("界", maximumSystemPromptLength+1),
	})
	if !errors.Is(err, ErrInvalidInstructions) {
		t.Fatalf("oversized prompt error = %v", err)
	}
}

type agentRepositoryStub struct {
	update InstructionsUpdate
}

func (*agentRepositoryStub) List(context.Context, string) ([]Agent, error)  { return nil, nil }
func (*agentRepositoryStub) Default(context.Context, string) (Agent, error) { return Agent{}, nil }
func (*agentRepositoryStub) Get(context.Context, string, string) (Agent, error) {
	return Agent{}, nil
}
func (*agentRepositoryStub) GetInstructions(context.Context, string, string) (Instructions, error) {
	return Instructions{AgentID: "agent-1", SystemPrompt: "Be concise.", Version: 1, UpdatedAt: time.Now()}, nil
}
func (repository *agentRepositoryStub) UpdateInstructions(_ context.Context, _, _ string, update InstructionsUpdate) (Instructions, error) {
	repository.update = update
	return Instructions{AgentID: "agent-1", SystemPrompt: update.SystemPrompt, Version: update.ExpectedVersion + 1, UpdatedAt: time.Now()}, nil
}
