package model

import (
	"time"

	"github.com/google/uuid"
)

type Vote struct {
	ID              uuid.UUID `json:"id"`
	TopicID         uuid.UUID `json:"topic_id"`
	Username        string    `json:"username"`
	RawMessage      string    `json:"raw_message"`
	ClassifiedLabel string    `json:"classified_label"`
	Confidence      float64   `json:"confidence"`
	Weight          int       `json:"weight"`
	IsDonation      bool      `json:"is_donation"`
	BitsAmount      int       `json:"bits_amount"`
	CreatedAt       time.Time `json:"created_at"`
}

type SubmitVoteRequest struct {
	TopicID    uuid.UUID `json:"topic_id" binding:"required"`
	Username   string    `json:"username" binding:"required,min=1,max=100"`
	Message    string    `json:"message" binding:"required,min=1"`
	IsDonation bool      `json:"is_donation"`
	BitsAmount int       `json:"bits_amount"`
}

type LeaderboardEntry struct {
	Label       string    `json:"label"`
	TotalWeight int       `json:"total_weight"`
	VoteCount   int       `json:"vote_count"`
	LastVoteAt  time.Time `json:"last_vote_at"`
}

type Leaderboard struct {
	TopicID   uuid.UUID          `json:"topic_id"`
	Topic     string             `json:"topic"`
	Entries   []LeaderboardEntry `json:"entries"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type MergeLabelsRequest struct {
	TopicID      uuid.UUID `json:"topic_id" binding:"required"`
	SourceLabels []string  `json:"source_labels" binding:"required,min=1"`
	TargetLabel  string    `json:"target_label" binding:"required,min=1"`
}

type MergeLabelsResponse struct {
	TopicID       uuid.UUID `json:"topic_id"`
	MergedLabels  []string  `json:"merged_labels"`
	TargetLabel   string    `json:"target_label"`
	VotesAffected int       `json:"votes_affected"`
}
