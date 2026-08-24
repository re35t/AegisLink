package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/conversation"
)

const agUIEventPollInterval = 100 * time.Millisecond

type agUIRunAgentInput struct {
	ThreadID       string             `json:"threadId"`
	RunID          string             `json:"runId"`
	Messages       []agUIMessage      `json:"messages"`
	State          json.RawMessage    `json:"state"`
	ForwardedProps agUIForwardedProps `json:"forwardedProps"`
}

type agUIForwardedProps struct {
	AegisLink *struct {
		Selection *conversation.RunSelection `json:"selection"`
	} `json:"aegislink"`
}

type agUIMessage struct {
	ID      string          `json:"id"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type agUITextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (handler *handler) runAgent(c *gin.Context) {
	var input agUIRunAgentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_ag_ui_input", "request body must be a valid AG-UI RunAgentInput")
		return
	}
	messageID, content, err := input.latestUserText()
	if err != nil {
		writeError(c, http.StatusBadRequest, "unsupported_ag_ui_input", err.Error())
		return
	}
	principalID := actorFrom(c).User.ID
	var selection *conversation.RunSelection
	if input.ForwardedProps.AegisLink != nil {
		selection = input.ForwardedProps.AegisLink.Selection
	}
	_, run, err := handler.conversations.StartRun(c.Request.Context(), principalID, conversation.RunRequest{
		ConversationID: input.ThreadID,
		MessageID:      messageID,
		RunID:          input.RunID,
		Content:        content,
		Selection:      selection,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	handler.streamAGUI(c, principalID, run)
}

func (input agUIRunAgentInput) latestUserText() (string, string, error) {
	if input.ThreadID == "" || input.RunID == "" || len(input.Messages) == 0 {
		return "", "", fmt.Errorf("threadId, runId, and messages are required")
	}
	message := input.Messages[len(input.Messages)-1]
	if message.ID == "" || message.Role != "user" {
		return "", "", fmt.Errorf("the final AG-UI message must be a user message with an id")
	}
	var text string
	if err := json.Unmarshal(message.Content, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "", "", fmt.Errorf("the final user message must contain text")
		}
		return message.ID, text, nil
	}
	var parts []agUITextPart
	if err := json.Unmarshal(message.Content, &parts); err != nil || len(parts) == 0 {
		return "", "", fmt.Errorf("the final user message content must be text")
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" {
			return "", "", fmt.Errorf("multimodal AG-UI input is not supported yet")
		}
		if strings.TrimSpace(part.Text) != "" {
			texts = append(texts, part.Text)
		}
	}
	text = strings.Join(texts, "\n")
	if strings.TrimSpace(text) == "" {
		return "", "", fmt.Errorf("the final user message must contain text")
	}
	return message.ID, text, nil
}

func (handler *handler) streamAGUI(c *gin.Context, principalID string, run conversation.Run) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	terminal := false
	defer func() {
		if !terminal {
			_ = handler.conversations.CancelRun(context.WithoutCancel(c.Request.Context()), principalID, run.ID)
		}
	}()

	if !writeAGUIEvent(c, map[string]any{
		"type": "RUN_STARTED", "threadId": run.ConversationID, "runId": run.ID,
	}) {
		return
	}

	messageID := conversation.AssistantMessageID(run.ID)
	messageStarted := false
	after := int64(0)
	ticker := time.NewTicker(agUIEventPollInterval)
	defer ticker.Stop()

	for {
		events, err := handler.conversations.ListRunEvents(c.Request.Context(), principalID, run.ID, after)
		if err != nil {
			writeAGUIEvent(c, map[string]any{"type": "RUN_ERROR", "message": "run event stream failed", "code": "event_stream_failed"})
			return
		}
		for _, event := range events {
			after = event.Sequence
			switch event.Type {
			case "tool.started":
				var payload struct {
					ToolCallID string `json:"toolCallId"`
					ToolName   string `json:"toolName"`
					Arguments  string `json:"arguments"`
				}
				if json.Unmarshal(event.Payload, &payload) != nil || payload.ToolCallID == "" || payload.ToolName == "" {
					continue
				}
				if !writeAGUIEvent(c, map[string]any{
					"type": "TOOL_CALL_START", "toolCallId": payload.ToolCallID,
					"toolCallName": payload.ToolName, "parentMessageId": messageID,
				}) {
					return
				}
				if payload.Arguments != "" && !writeAGUIEvent(c, map[string]any{
					"type": "TOOL_CALL_ARGS", "toolCallId": payload.ToolCallID, "delta": payload.Arguments,
				}) {
					return
				}
				if !writeAGUIEvent(c, map[string]any{"type": "TOOL_CALL_END", "toolCallId": payload.ToolCallID}) {
					return
				}
			case "tool.completed":
				var payload struct {
					ToolCallID string `json:"toolCallId"`
					Result     string `json:"result"`
				}
				if json.Unmarshal(event.Payload, &payload) != nil || payload.ToolCallID == "" {
					continue
				}
				if !writeAGUIEvent(c, map[string]any{
					"type": "TOOL_CALL_RESULT", "messageId": payload.ToolCallID + ":result",
					"toolCallId": payload.ToolCallID, "content": payload.Result, "role": "tool",
				}) {
					return
				}
			case "tool.failed":
				var payload struct {
					ToolCallID string `json:"toolCallId"`
					Error      string `json:"error"`
				}
				if json.Unmarshal(event.Payload, &payload) != nil || payload.ToolCallID == "" {
					continue
				}
				if !writeAGUIEvent(c, map[string]any{
					"type": "TOOL_CALL_RESULT", "messageId": payload.ToolCallID + ":result",
					"toolCallId": payload.ToolCallID, "content": payload.Error, "role": "tool", "isError": true,
				}) {
					return
				}
			case "message.delta":
				if !messageStarted {
					messageStarted = writeAGUIEvent(c, map[string]any{
						"type": "TEXT_MESSAGE_START", "messageId": messageID, "role": "assistant",
					})
					if !messageStarted {
						return
					}
				}
				var payload struct {
					Delta string `json:"delta"`
				}
				if json.Unmarshal(event.Payload, &payload) != nil || payload.Delta == "" {
					continue
				}
				if !writeAGUIEvent(c, map[string]any{
					"type": "TEXT_MESSAGE_CONTENT", "messageId": messageID, "delta": payload.Delta,
				}) {
					return
				}
			case "message.completed":
				if !messageStarted {
					messageStarted = writeAGUIEvent(c, map[string]any{
						"type": "TEXT_MESSAGE_START", "messageId": messageID, "role": "assistant",
					})
					if !messageStarted {
						return
					}
					var payload struct {
						Message conversation.Message `json:"message"`
					}
					if json.Unmarshal(event.Payload, &payload) == nil && payload.Message.Content != "" {
						if !writeAGUIEvent(c, map[string]any{
							"type": "TEXT_MESSAGE_CONTENT", "messageId": messageID, "delta": payload.Message.Content,
						}) {
							return
						}
					}
				}
				if !writeAGUIEvent(c, map[string]any{"type": "TEXT_MESSAGE_END", "messageId": messageID}) {
					return
				}
				terminal = writeAGUIEvent(c, map[string]any{
					"type": "RUN_FINISHED", "threadId": run.ConversationID, "runId": run.ID,
					"outcome": map[string]string{"type": "success"},
				})
				return
			case "run.failed", "run.cancelled":
				if messageStarted && !writeAGUIEvent(c, map[string]any{"type": "TEXT_MESSAGE_END", "messageId": messageID}) {
					return
				}
				var payload struct {
					Code string `json:"code"`
				}
				_ = json.Unmarshal(event.Payload, &payload)
				terminal = writeAGUIEvent(c, map[string]any{
					"type": "RUN_ERROR", "message": "agent run did not complete", "code": payload.Code,
				})
				return
			}
		}

		current, err := handler.conversations.GetRun(c.Request.Context(), principalID, run.ID)
		if err != nil {
			return
		}
		if current.Terminal() {
			// A terminal event and status are committed together, but the status can
			// become visible between this iteration's event query and GetRun.
			// Loop once more so the event is never dropped at that boundary.
			continue
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func writeAGUIEvent(c *gin.Context, event any) bool {
	encoded, err := json.Marshal(event)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", encoded); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}
