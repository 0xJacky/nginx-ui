package llm

import (
	"net/http"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const TerminalAssistantPath = "__terminal_assistant__"

// GetLLMSessions returns LLM sessions with optional filtering
func GetLLMSessions(c *gin.Context) {
	g := query.LLMSession
	query := g.Order(g.UpdatedAt.Desc())
	
	// Filter by type if provided
	if assistantType := c.Query("type"); assistantType != "" {
		if assistantType == "terminal" {
			// For terminal type, filter by terminal assistant path
			query = query.Where(g.Path.Eq(TerminalAssistantPath))
		} else if assistantType == "nginx" {
			// For nginx type, exclude terminal assistant path
			query = query.Where(g.Path.Neq(TerminalAssistantPath))
		}
	} else if path := c.Query("path"); path != "" {
		// Filter by path if provided (legacy support)
		// Skip path validation for terminal assistant
		if path != TerminalAssistantPath && !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "path is not under the nginx conf path",
			})
			return
		}
		query = query.Where(g.Path.Eq(path))
	}
	
	sessions, err := query.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// GetLLMSession returns a single session by session_id
func GetLLMSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	
	g := query.LLMSession
	session, err := g.Where(g.SessionID.Eq(sessionID)).First()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// CreateLLMSession creates a new LLM session
func CreateLLMSession(c *gin.Context) {
	var json struct {
		Title string `json:"title" binding:"required"`
		Path  string `json:"path"`
		Type  string `json:"type"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Determine path based on type
	var sessionPath string
	if json.Type == "terminal" {
		sessionPath = TerminalAssistantPath
	} else {
		sessionPath = json.Path
		// Validate path for non-terminal types
		if sessionPath != "" && !helper.IsUnderDirectory(sessionPath, nginx.GetConfPath()) {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "path is not under the nginx conf path",
			})
			return
		}
	}

	session := &model.LLMSession{
		Title:        json.Title,
		Path:         sessionPath,
		Messages:     []openai.ChatCompletionMessage{},
		MessageCount: 0,
		IsActive:     true,
	}

	g := query.LLMSession
	
	// When creating a new active session, deactivate all other sessions with the same path
	if session.IsActive && sessionPath != "" {
		_, err := g.Where(g.Path.Eq(sessionPath)).UpdateSimple(g.IsActive.Value(false))
		if err != nil {
			logger.Error("Failed to deactivate other sessions:", err)
			// Continue anyway, this is not critical
		}
	}
	
	err := g.Create(session)
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// UpdateLLMSession updates an existing session
func UpdateLLMSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	
	var json struct {
		Title    string                         `json:"title,omitempty"`
		Messages []openai.ChatCompletionMessage `json:"messages,omitempty"`
		IsActive *bool                          `json:"is_active,omitempty"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	g := query.LLMSession
	session, err := g.Where(g.SessionID.Eq(sessionID)).First()
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	// Update fields
	if json.Title != "" {
		session.Title = json.Title
	}
	
	if json.Messages != nil {
		session.Messages = json.Messages
		session.MessageCount = len(json.Messages)
	}
	
	if json.IsActive != nil && *json.IsActive {
		session.IsActive = true
		
		// Deactivate all other sessions with the same path
		_, err = g.Where(
			g.Path.Eq(session.Path),
			g.SessionID.Neq(sessionID),
		).UpdateSimple(g.IsActive.Value(false))
		
		if err != nil {
			logger.Error("Failed to deactivate other sessions:", err)
			// Continue anyway, this is not critical
		}
	} else if json.IsActive != nil {
		session.IsActive = *json.IsActive
	}

	// Save the updated session
	err = g.Save(session)
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteLLMSession deletes a session by session_id
func DeleteLLMSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	
	g := query.LLMSession
	result, err := g.Where(g.SessionID.Eq(sessionID)).Delete()
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Session not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Session deleted successfully",
	})
}

// DuplicateLLMSession duplicates an existing session
func DuplicateLLMSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	
	g := query.LLMSession
	originalSession, err := g.Where(g.SessionID.Eq(sessionID)).First()
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	// Create a new session with the same content
	newSession := &model.LLMSession{
		Title:        originalSession.Title + " (Copy)",
		Path:         originalSession.Path,
		Messages:     originalSession.Messages,
		MessageCount: originalSession.MessageCount,
	}

	err = g.Create(newSession)
	if err != nil {
		logger.Error(err)
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, newSession)
}

// GetLLMSessionByPath - 兼容性端点，基于路径获取或创建会话
func GetLLMSessionByPath(c *gin.Context) {
	path := c.Query("path")

	// Skip path validation for terminal assistant
	if path != TerminalAssistantPath && !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	g := query.LLMSession
	
	// 查找基于该路径的会话
	session, err := g.Where(g.Path.Eq(path)).First()
	if err != nil {
		// 如果没找到，创建一个新的会话
		title := "Chat for " + filepath.Base(path)
		session = &model.LLMSession{
			Title:        title,
			Path:         path,
			Messages:     []openai.ChatCompletionMessage{},
			MessageCount: 0,
			IsActive:     true,
		}
		
		// Deactivate all other sessions with the same path before creating
		if path != "" {
			_, deactivateErr := g.Where(g.Path.Eq(path)).UpdateSimple(g.IsActive.Value(false))
			if deactivateErr != nil {
				logger.Error("Failed to deactivate other sessions:", deactivateErr)
			}
		}
		
		err = g.Create(session)
		if err != nil {
			logger.Error(err)
			cosy.ErrHandler(c, err)
			return
		}
	}

	// 返回兼容格式
	response := struct {
		Name    string                         `json:"name"`
		Content []openai.ChatCompletionMessage `json:"content"`
	}{
		Name:    session.Path,
		Content: session.Messages,
	}

	c.JSON(http.StatusOK, response)
}

// CreateOrUpdateLLMSessionByPath - 兼容性端点，基于路径创建或更新会话
func CreateOrUpdateLLMSessionByPath(c *gin.Context) {
	var json struct {
		FileName string                         `json:"file_name"`
		Messages []openai.ChatCompletionMessage `json:"messages"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Skip path validation for terminal assistant
	if json.FileName != TerminalAssistantPath && !helper.IsUnderDirectory(json.FileName, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	g := query.LLMSession
	
	// 查找或创建基于该路径的会话
	session, err := g.Where(g.Path.Eq(json.FileName)).First()
	if err != nil {
		// 创建新会话
		title := "Chat for " + filepath.Base(json.FileName)
		session = &model.LLMSession{
			Title:        title,
			Path:         json.FileName,
			Messages:     json.Messages,
			MessageCount: len(json.Messages),
			IsActive:     true,
		}
		
		// Deactivate all other sessions with the same path before creating
		if json.FileName != "" {
			_, deactivateErr := g.Where(g.Path.Eq(json.FileName)).UpdateSimple(g.IsActive.Value(false))
			if deactivateErr != nil {
				logger.Error("Failed to deactivate other sessions:", deactivateErr)
			}
		}
		
		err = g.Create(session)
		if err != nil {
			logger.Error(err)
			cosy.ErrHandler(c, err)
			return
		}
	} else {
		// 更新现有会话
		session.Messages = json.Messages
		session.MessageCount = len(json.Messages)
		
		err = g.Save(session)
		if err != nil {
			logger.Error(err)
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

// GenerateTitle generates a title based on messages (runs on main node only)
func GenerateTitle(c *gin.Context) {
	var json struct {
		Messages []openai.ChatCompletionMessage `json:"messages" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	title, err := llm.GenerateSessionTitle(json.Messages)
	if err != nil {
		logger.Error("Failed to generate title:", err)
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"title": title,
	})
}