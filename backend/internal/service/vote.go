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
	MergeLabels(ctx context.Context, req *model.MergeLabelsRequest) (*model.MergeLabelsResponse, error)
}

type voteService struct {
	topicRepo   repository.TopicRepository
	voteRepo    repository.VoteRepository
	processor   *VoteProcessor
	tallyCache  *VoteTallyCache
	wsHub       WSBroadcaster
	threshold   float64
}

func NewVoteService(
	topicRepo repository.TopicRepository,
	voteRepo repository.VoteRepository,
	processor *VoteProcessor,
	tallyCache *VoteTallyCache,
	wsHub WSBroadcaster,
	threshold float64,
) VoteService {
	return &voteService{
		topicRepo:  topicRepo,
		voteRepo:   voteRepo,
		processor:  processor,
		tallyCache: tallyCache,
		wsHub:      wsHub,
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

func (s *voteService) MergeLabels(ctx context.Context, req *model.MergeLabelsRequest) (*model.MergeLabelsResponse, error) {
	topic, err := s.topicRepo.GetByID(ctx, req.TopicID)
	if err != nil {
		return nil, fmt.Errorf("get topic: %w", err)
	}
	if topic == nil {
		return nil, ErrTopicNotFound
	}

	validSources := make([]string, 0, len(req.SourceLabels))
	for _, src := range req.SourceLabels {
		if src != req.TargetLabel {
			validSources = append(validSources, src)
		}
	}
	if len(validSources) == 0 {
		return nil, ErrNoLabelsToMerge
	}

	affected, err := s.voteRepo.MergeLabels(ctx, req.TopicID, validSources, req.TargetLabel)
	if err != nil {
		return nil, fmt.Errorf("merge labels in db: %w", err)
	}

	s.tallyCache.MergeLabels(req.TopicID, validSources, req.TargetLabel)

	go s.wsHub.BroadcastLeaderboard(req.TopicID)

	return &model.MergeLabelsResponse{
		TopicID:       req.TopicID,
		MergedLabels:  validSources,
		TargetLabel:   req.TargetLabel,
		VotesAffected: affected,
	}, nil
}
