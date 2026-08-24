package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/re35t/AegisLink/internal/skills"
)

const skillMultipartOverheadBytes = 512 * 1024

func (handler *handler) listSkills(c *gin.Context) {
	actor := actorFrom(c)
	items, err := handler.skills.List(c.Request.Context(), actor.User.ID, c.Param("agentId"))
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"skills": items})
}

func (handler *handler) installSkill(c *gin.Context) {
	var body struct {
		Content string `json:"content" binding:"required"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_skill", "SKILL.md content is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.skills.Install(c.Request.Context(), actor.User.ID, c.Param("agentId"), body.Content, body.Version)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) importSkill(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		skills.MaximumImportBytes+skillMultipartOverheadBytes,
	)
	if err := c.Request.ParseMultipartForm(1024 * 1024); err != nil {
		var maximumBytesError *http.MaxBytesError
		if errors.As(err, &maximumBytesError) {
			handler.handleError(c, skills.ErrTooLarge)
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_skill_bundle", "bundle must be a SKILL.md or ZIP file")
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	fileHeaders := c.Request.MultipartForm.File["bundle"]
	if len(fileHeaders) != 1 {
		writeError(c, http.StatusBadRequest, "invalid_skill_bundle", "exactly one bundle file is required")
		return
	}
	fileHeader := fileHeaders[0]
	if fileHeader.Size > skills.MaximumImportBytes {
		handler.handleError(c, skills.ErrTooLarge)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_skill_bundle", "the uploaded bundle could not be read")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, skills.MaximumImportBytes+1))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_skill_bundle", "the uploaded bundle could not be read")
		return
	}
	if len(data) > skills.MaximumImportBytes {
		handler.handleError(c, skills.ErrTooLarge)
		return
	}
	version := ""
	if values := c.Request.MultipartForm.Value["version"]; len(values) > 0 {
		version = values[0]
	}
	actor := actorFrom(c)
	item, err := handler.skills.Import(c.Request.Context(), actor.User.ID, c.Param("agentId"), skills.ImportRequest{
		FileName: fileHeader.Filename,
		Data:     data,
		Version:  version,
	})
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (handler *handler) updateSkill(c *gin.Context) {
	var body struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		writeError(c, http.StatusBadRequest, "invalid_skill", "enabled is required")
		return
	}
	actor := actorFrom(c)
	item, err := handler.skills.SetEnabled(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("skillId"), *body.Enabled)
	if err != nil {
		handler.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (handler *handler) uninstallSkill(c *gin.Context) {
	actor := actorFrom(c)
	if err := handler.skills.Uninstall(c.Request.Context(), actor.User.ID, c.Param("agentId"), c.Param("skillId")); err != nil {
		handler.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
