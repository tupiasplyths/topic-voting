package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/repository"
)

type VoteService interface {
	SubmitVote(ctx context.Context, req *model.SubmitVoteRequest) (int, error)
	GetLeaderboard(ctx context.Context, topicID uuid.UUID, limit int) (*model.Leaderboard, error)
	GetLabels(topicID uuid.UUID) []string
}

type voteService struct {
	topicRepo   repository.TopicRepository
	processor   *VoteProcessor
	tallyCache  *VoteTallyCache
	threshold   float64
}

func NewVoteService(
	topicRepo repository.TopicRepository,
	processor *VoteProcessor,
	tallyCache *VoteTallyCache,
	threshold float64,
) VoteService {
	return &voteService{
		topicRepo:  topicRepo,
		processor:  processor,
		tallyCache: tallyCache,
		threshold:  threshold,
	}
}

func (s *voteService) SubmitVote(ctx context.Context, req *model.SubmitVoteRequest) (int, error) {
	topic, err := s.topicRepo.GetByID(ctx, req.TopicID)
	if err != nil {
		return 0, fmt.Errorf("get topic: %w", err)
	}
	if topic == nil {
		return 0, ErrTopicNotFound
	}
	if !topic.IsActive {
		return 0, ErrTopicNotActive
	}

	threshold := s.threshold
	if topic.ClassifierThreshold > 0 {
		threshold = topic.ClassifierThreshold
	}

	weight := computeWeight(req.IsDonation, req.BitsAmount)

	pv := &PendingVote{
		TopicID:    req.TopicID,
		TopicTitle: topic.Title,
		Username:   req.Username,
		Message:    req.Message,
		IsDonation: req.IsDonation,
		BitsAmount: req.BitsAmount,
		Threshold:  threshold,
	}

	if err := s.processor.Enqueue(pv); err != nil {
		return 0, err
	}

	return weight, nil
}

func (s *voteService) GetLeaderboard(_ context.Context, topicID uuid.UUID, limit int) (*model.Leaderboard, error) {
	lb, err := s.tallyCache.GetLeaderboard(topicID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(lb.Entries) > limit {
		lb.Entries = lb.Entries[:limit]
	}
	return lb, nil
}

func (s *voteService) GetLabels(topicID uuid.UUID) []string {
	return s.tallyCache.GetLabels(topicID)
}
