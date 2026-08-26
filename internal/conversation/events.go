package conversation

func persistedToolEvent(event *HarnessToolEvent) (string, map[string]any, bool) {
	if event == nil || event.ID == "" {
		return "", nil, false
	}
	switch event.Type {
	case HarnessToolStarted:
		if event.Name == "" {
			return "", nil, false
		}
		return "tool.started", map[string]any{
			"toolCallId": event.ID,
			"toolName":   event.Name,
			"arguments":  event.Arguments,
		}, true
	case HarnessToolCompleted:
		return "tool.completed", map[string]any{
			"toolCallId": event.ID,
			"result":     event.Result,
		}, true
	case HarnessToolFailed:
		return "tool.failed", map[string]any{
			"toolCallId": event.ID,
			"error":      event.Error,
		}, true
	default:
		return "", nil, false
	}
}

func AssistantMessageID(runID string) string {
	return runID + ":assistant"
}
