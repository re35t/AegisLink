package conversation

func persistedToolEvent(event *RuntimeToolEvent) (string, map[string]any, bool) {
	if event == nil || event.ID == "" {
		return "", nil, false
	}
	switch event.Type {
	case RuntimeToolStarted:
		if event.Name == "" {
			return "", nil, false
		}
		return "tool.started", map[string]any{
			"toolCallId": event.ID,
			"toolName":   event.Name,
			"arguments":  event.Arguments,
		}, true
	case RuntimeToolCompleted:
		return "tool.completed", map[string]any{
			"toolCallId": event.ID,
			"result":     event.Result,
		}, true
	case RuntimeToolFailed:
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
