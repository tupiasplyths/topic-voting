package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/service"
)

type TopicHandler struct {
	svc service.TopicService
}

func NewTopicHandler(svc service.TopicService) *TopicHandler {
	return &TopicHandler{svc: svc}
}

func (h *TopicHandler) RegisterRoutes(rg *gin.RouterGroup) {
	topics := rg.Group("/topics")
	{
		topics.POST("", h.Create)
		topics.GET("", h.List)
		topics.GET("/active", h.GetActive)
		topics.POST("/:id/close", h.Close)
	}
}

func (h *TopicHandler) Create(c *gin.Context) {
	var req model.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": err.Error()})
		return
	}

	topic, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidThreshold) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusCreated, topic)
}

func (h *TopicHandler) List(c *gin.Context) {
	topics, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	if topics == nil {
		topics = []*model.Topic{}
	}
	c.JSON(http.StatusOK, topics)
}

func (h *TopicHandler) GetActive(c *gin.Context) {
	topic, err := h.svc.GetActive(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrNoActiveTopic) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no_active_topic"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, topic)
}

func (h *TopicHandler) Close(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": "invalid topic id"})
		return
	}

	topic, err := h.svc.Close(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrTopicNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "topic_not_found"})
			return
		}
		if errors.Is(err, service.ErrTopicNotActive) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "topic_not_active"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        topic.ID,
		"is_active": topic.IsActive,
		"closed_at": topic.ClosedAt,
	})
}