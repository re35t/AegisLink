package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/account"
)

const actorContextKey = "authenticatedActor"

func authenticate(accounts AccountService, cookieName string, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil || token == "" {
			writeError(c, http.StatusUnauthorized, "authentication_required", "sign in to continue")
			return
		}
		actor, err := accounts.Authenticate(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, account.ErrUnauthenticated) {
				writeError(c, http.StatusUnauthorized, "authentication_required", "sign in to continue")
				return
			}
			logger.Error("authenticate request", "requestId", requestIDFrom(c), "error", err)
			writeError(c, http.StatusInternalServerError, "internal_error", "the server could not complete the request")
			return
		}
		c.Set(actorContextKey, actor)
		c.Next()
	}
}

func actorFrom(c *gin.Context) account.Actor {
	value, _ := c.Get(actorContextKey)
	actor, _ := value.(account.Actor)
	return actor
}
