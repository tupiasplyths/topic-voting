package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/middleware"
	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/service"
)

type VoteHandler struct {
	svc service.VoteService
}

func NewVoteHandler(svc service.VoteService) *VoteHandler {
	return &VoteHandler{svc: svc}
}

func (h *VoteHandler) RegisterRoutes(rg *gin.RouterGroup, adminKey string) {
	votes := rg.Group("/votes")
	{
		votes.POST("", h.Submit)
		votes.GET("/leaderboard", h.GetLeaderboard)
		votes.GET("/labels", h.GetLabels)
		votes.POST("/merge-labels", middleware.RequireAdminKey(adminKey), h.MergeLabels)
	}
}

func (h *VoteHandler) Submit(c *gin.Context) {
	var req model.SubmitVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": err.Error()})
		return
	}

	weight, err := h.svc.SubmitVote(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrTopicNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "topic_not_found"})
			return
		}
		if errors.Is(err, service.ErrTopicNotActive) {
			c.JSON(http.StatusNotFound, gin.H{"error": "topic_not_active"})
			return
		}
		if errors.Is(err, service.ErrQueueFull) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":           "queue_full",
				"retry_after_ms": 500,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":   "queued",
		"topic_id": req.TopicID,
		"username": req.Username,
		"weight":   weight,
	})
}

func (h *VoteHandler) GetLeaderboard(c *gin.Context) {
	topicIDStr := c.Query("topic_id")
	if topicIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": "topic_id is required"})
		return
	}
	topicID, err := uuid.Parse(topicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": "invalid topic_id"})
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	lb, err := h.svc.GetLeaderboard(c.Request.Context(), topicID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	c.JSON(http.StatusOK, lb)
}

func (h *VoteHandler) GetLabels(c *gin.Context) {
	topicIDStr := c.Query("topic_id")
	if topicIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": "topic_id is required"})
		return
	}
	topicID, err := uuid.Parse(topicIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": "invalid topic_id"})
		return
	}

	labels := h.svc.GetLabels(topicID)
	if labels == nil {
		labels = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"topic_id": topicID, "labels": labels})
}

func (h *VoteHandler) MergeLabels(c *gin.Context) {
	var req model.MergeLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation", "details": err.Error()})
		return
	}

	if len(req.SourceLabels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation",
			"details": "source_labels must not be empty",
		})
		return
	}

	resp, err := h.svc.MergeLabels(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrTopicNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "topic_not_found"})
			return
		}
		if errors.Is(err, service.ErrNoLabelsToMerge) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no_labels_to_merge"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
