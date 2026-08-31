package conversation

import (
	"context"
	"errors"
	"strings"
)

func (service *Service) execute(ctx context.Context, principalID string, run Run) {
	defer service.removeActive(run.ID)
	if err := service.repository.MarkRunRunning(ctx, principalID, run.ID); err != nil {
		service.logger.Error("mark run running", "runId", run.ID, "error", err)
		return
	}
	if _, err := service.repository.AppendRunEvent(ctx, principalID, run.ID, "run.started", map[string]any{"executionPolicy": run.ExecutionPolicy}); err != nil {
		service.finishFailed(principalID, run.ID, "event_persistence_failed", false)
		return
	}

	detail, err := service.repository.GetConversation(ctx, principalID, run.ConversationID)
	if err != nil {
		service.finishFailed(principalID, run.ID, "conversation_load_failed", false)
		return
	}
	agentRecord, err := service.agents.Get(ctx, principalID, detail.Conversation.AgentID)
	if err != nil {
		service.finishFailed(principalID, run.ID, "agent_load_failed", false)
		return
	}
	outputs, err := service.harness.Run(ctx, HarnessInput{RunID: run.ID, PrincipalID: principalID, Agent: agentRecord, Messages: detail.Messages, Policy: run.ExecutionPolicy})
	if err != nil {
		service.logger.Error("prepare agent harness", "runId", run.ID, "error", err)
		service.finishFailed(principalID, run.ID, "agent_context_load_failed", false)
		return
	}

	var answer strings.Builder
	for output := range outputs {
		if output.Err != nil {
			if errors.Is(output.Err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				service.finishFailed(principalID, run.ID, "cancelled_by_user", true)
			} else if errors.Is(output.Err, ErrForcedToolNotCalled) {
				service.finishFailed(principalID, run.ID, "forced_tool_not_called", false)
			} else if errors.Is(output.Err, ErrForcedToolMismatch) {
				service.finishFailed(principalID, run.ID, "forced_tool_mismatch", false)
			} else if errors.Is(output.Err, ErrSelectedSkillMismatch) {
				service.finishFailed(principalID, run.ID, "selected_skill_mismatch", false)
			} else {
				service.logger.Error("agent runtime failed", "runId", run.ID, "error", output.Err)
				service.finishFailed(principalID, run.ID, "runtime_error", false)
			}
			return
		}
		if output.Tool != nil {
			eventType, payload, ok := persistedToolEvent(output.Tool)
			if !ok {
				service.logger.Error("invalid runtime tool event", "runId", run.ID, "toolCallId", output.Tool.ID, "type", output.Tool.Type)
				service.finishFailed(principalID, run.ID, "invalid_runtime_event", false)
				return
			}
			if _, err := service.repository.AppendRunEvent(ctx, principalID, run.ID, eventType, payload); err != nil {
				service.finishFailed(principalID, run.ID, "event_persistence_failed", false)
				return
			}
			continue
		}
		if output.Delta == "" {
			continue
		}
		answer.WriteString(output.Delta)
		if _, err := service.repository.AppendRunEvent(ctx, principalID, run.ID, "message.delta", map[string]any{"delta": output.Delta}); err != nil {
			service.finishFailed(principalID, run.ID, "event_persistence_failed", false)
			return
		}
	}

	if err := ctx.Err(); err != nil {
		service.finishFailed(principalID, run.ID, "cancelled_by_user", true)
		return
	}
	if answer.Len() == 0 {
		service.finishFailed(principalID, run.ID, "empty_model_response", false)
		return
	}
	if _, err := service.repository.CompleteRun(
		context.WithoutCancel(ctx),
		principalID,
		run.ID,
		run.ConversationID,
		AssistantMessageID(run.ID),
		answer.String(),
	); err != nil {
		service.logger.Error("complete run", "runId", run.ID, "error", err)
		service.finishFailed(principalID, run.ID, "result_persistence_failed", false)
	}
}

func (service *Service) finishFailed(principalID, runID, code string, cancelled bool) {
	if err := service.repository.FailRun(context.WithoutCancel(service.root), principalID, runID, code, cancelled); err != nil {
		service.logger.Error("finish failed run", "runId", runID, "failureCode", code, "error", err)
	}
}

func (service *Service) removeActive(runID string) {
	service.activeMu.Lock()
	delete(service.active, runID)
	service.activeMu.Unlock()
}
