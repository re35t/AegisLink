package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/account"
	"github.com/re35t/AegisLink/internal/agent"
)

func (handler *handler) register(c *gin.Context) {
	var body struct {
		DisplayName string `json:"displayName" binding:"required"`
		Email       string `json:"email" binding:"required"`
		Password    string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_registration", "displayName, email, and password are required")
		return
	}
	result, err := handler.accounts.Register(c.Request.Context(), body.DisplayName, body.Email, body.Password)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	handler.setSessionCookie(c, result.SessionToken)
	c.JSON(http.StatusCreated, authResponse(result.User, result.Agent))
}

func (handler *handler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_login", "email and password are required")
		return
	}
	result, err := handler.accounts.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	handler.setSessionCookie(c, result.SessionToken)
	c.JSON(http.StatusOK, authResponse(result.User, result.Agent))
}

func (handler *handler) logout(c *gin.Context) {
	token, _ := c.Cookie(handler.auth.CookieName)
	if err := handler.accounts.Logout(c.Request.Context(), token); err != nil {
		handler.handleError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: handler.auth.CookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: handler.auth.CookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
	c.Status(http.StatusNoContent)
}

func (handler *handler) session(c *gin.Context) {
	actor := actorFrom(c)
	agentRecord, err := handler.agents.Bootstrap(c.Request.Context(), actor.User.ID)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, authResponse(actor.User, agentRecord))
}

func (handler *handler) setSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: handler.auth.CookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: handler.auth.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func authResponse(user account.User, agentRecord agent.Agent) gin.H {
	return gin.H{"user": user, "agent": agentRecord}
}
