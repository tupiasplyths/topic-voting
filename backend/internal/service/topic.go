package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/repository"
)

type TopicService interface {
	Create(ctx context.Context, req *model.CreateTopicRequest) (*model.Topic, error)
	List(ctx context.Context) ([]*model.Topic, error)
	GetActive(ctx context.Context) (*model.Topic, error)
	Close(ctx context.Context, id uuid.UUID) (*model.Topic, error)
}

type topicService struct {
	repo repository.TopicRepository
}

func NewTopicService(repo repository.TopicRepository) TopicService {
	return &topicService{repo: repo}
}

func (s *topicService) Create(ctx context.Context, req *model.CreateTopicRequest) (*model.Topic, error) {
	threshold := 0.5
	if req.ClassifierThreshold != nil {
		if *req.ClassifierThreshold <= 0 || *req.ClassifierThreshold > 1.0 {
			return nil, fmt.Errorf("classifier_threshold must be between 0 and 1.0: %w", ErrInvalidThreshold)
		}
		threshold = *req.ClassifierThreshold
	}

	topic := &model.Topic{
		Title:               req.Title,
		Description:         req.Description,
		IsActive:            req.SetActive,
		ClassifierThreshold: threshold,
	}

	if !req.SetActive {
		return s.repo.Create(ctx, topic)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeactivateAllWithTx(ctx, tx); err != nil {
		return nil, fmt.Errorf("deactivate existing topics: %w", err)
	}

	result, err := s.repo.CreateWithTx(ctx, tx, topic)
	if err != nil {
		return nil, fmt.Errorf("create topic: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

func (s *topicService) List(ctx context.Context) ([]*model.Topic, error) {
	return s.repo.List(ctx)
}

func (s *topicService) GetActive(ctx context.Context) (*model.Topic, error) {
	topic, err := s.repo.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active topic: %w", err)
	}
	if topic == nil {
		return nil, ErrNoActiveTopic
	}
	return topic, nil
}

func (s *topicService) Close(ctx context.Context, id uuid.UUID) (*model.Topic, error) {
	topic, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get topic: %w", err)
	}
	if topic == nil {
		return nil, ErrTopicNotFound
	}
	if !topic.IsActive {
		return nil, ErrTopicNotActive
	}

	result, err := s.repo.Close(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("close topic: %w", err)
	}
	return result, nil
}