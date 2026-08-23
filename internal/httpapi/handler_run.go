package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (handler *handler) runEvents(c *gin.Context) {
	after, err := eventCursor(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_event_cursor", "Last-Event-ID must be a non-negative integer")
		return
	}
	runID := c.Param("runId")
	principalID := actorFrom(c).User.ID
	if _, err := handler.conversations.GetRun(c.Request.Context(), principalID, runID); err != nil {
		handler.handleError(c, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		events, err := handler.conversations.ListRunEvents(c.Request.Context(), principalID, runID, after)
		if err != nil {
			return
		}
		for _, event := range events {
			encoded, err := json.Marshal(event)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, encoded); err != nil {
				return
			}
			after = event.Sequence
			c.Writer.Flush()
		}
		run, err := handler.conversations.GetRun(c.Request.Context(), principalID, runID)
		if err != nil || run.Terminal() {
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (handler *handler) cancelRun(c *gin.Context) {
	if err := handler.conversations.CancelRun(c.Request.Context(), actorFrom(c).User.ID, c.Param("runId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "cancelling"})
}

func eventCursor(c *gin.Context) (int64, error) {
	value := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	if value == "" {
		value = strings.TrimSpace(c.Query("after"))
	}
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		return 0, errors.New("invalid cursor")
	}
	return cursor, nil
}
