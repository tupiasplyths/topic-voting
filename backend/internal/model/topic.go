package model

import (
	"time"

	"github.com/google/uuid"
)

type Topic struct {
	ID                  uuid.UUID  `json:"id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	IsActive            bool       `json:"is_active"`
	ClassifierThreshold float64    `json:"classifier_threshold"`
	CreatedAt           time.Time  `json:"created_at"`
	ClosedAt            *time.Time `json:"closed_at,omitempty"`
}

type CreateTopicRequest struct {
	Title               string   `json:"title" binding:"required,min=1,max=255"`
	Description         string   `json:"description"`
	ClassifierThreshold *float64 `json:"classifier_threshold,omitempty"`
	SetActive           bool     `json:"set_active"`
}
