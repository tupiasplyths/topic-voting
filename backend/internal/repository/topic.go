package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/topic-voting/backend/internal/model"
)

type TopicRepository interface {
	Create(ctx context.Context, topic *model.Topic) (*model.Topic, error)
	DeactivateAll(ctx context.Context) error
	List(ctx context.Context) ([]*model.Topic, error)
	GetActive(ctx context.Context) (*model.Topic, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Topic, error)
	Close(ctx context.Context, id uuid.UUID) (*model.Topic, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateWithTx(ctx context.Context, tx pgx.Tx, topic *model.Topic) (*model.Topic, error)
	DeactivateAllWithTx(ctx context.Context, tx pgx.Tx) error
}

type topicRepo struct {
	pool *pgxpool.Pool
}

func NewTopicRepository(pool *pgxpool.Pool) TopicRepository {
	return &topicRepo{pool: pool}
}

func (r *topicRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *topicRepo) CreateWithTx(ctx context.Context, tx pgx.Tx, topic *model.Topic) (*model.Topic, error) {
	created := &model.Topic{
		Title:               topic.Title,
		Description:         topic.Description,
		IsActive:            topic.IsActive,
		ClassifierThreshold: topic.ClassifierThreshold,
	}
	err := tx.QueryRow(ctx,
		`INSERT INTO topics (title, description, is_active, classifier_threshold)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		created.Title, created.Description, created.IsActive, created.ClassifierThreshold,
	).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert topic: %w", err)
	}
	return created, nil
}

func (r *topicRepo) DeactivateAllWithTx(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE topics SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("deactivate all topics: %w", err)
	}
	return nil
}

func (r *topicRepo) Create(ctx context.Context, topic *model.Topic) (*model.Topic, error) {
	created := &model.Topic{
		Title:               topic.Title,
		Description:         topic.Description,
		IsActive:            topic.IsActive,
		ClassifierThreshold: topic.ClassifierThreshold,
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO topics (title, description, is_active, classifier_threshold)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		created.Title, created.Description, created.IsActive, created.ClassifierThreshold,
	).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert topic: %w", err)
	}
	return created, nil
}

func (r *topicRepo) DeactivateAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `UPDATE topics SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("deactivate all topics: %w", err)
	}
	return nil
}

func (r *topicRepo) List(ctx context.Context) ([]*model.Topic, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, description, is_active, classifier_threshold, created_at, closed_at
		 FROM topics ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	topics := make([]*model.Topic, 0)
	for rows.Next() {
		t := &model.Topic{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.IsActive, &t.ClassifierThreshold, &t.CreatedAt, &t.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

func (r *topicRepo) GetActive(ctx context.Context) (*model.Topic, error) {
	t := &model.Topic{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, description, is_active, classifier_threshold, created_at, closed_at
		 FROM topics WHERE is_active = TRUE ORDER BY created_at DESC LIMIT 1`,
	).Scan(&t.ID, &t.Title, &t.Description, &t.IsActive, &t.ClassifierThreshold, &t.CreatedAt, &t.ClosedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active topic: %w", err)
	}
	return t, nil
}

func (r *topicRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Topic, error) {
	t := &model.Topic{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, description, is_active, classifier_threshold, created_at, closed_at
		 FROM topics WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.IsActive, &t.ClassifierThreshold, &t.CreatedAt, &t.ClosedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get topic by id: %w", err)
	}
	return t, nil
}

func (r *topicRepo) Close(ctx context.Context, id uuid.UUID) (*model.Topic, error) {
	t := &model.Topic{}
	err := r.pool.QueryRow(ctx,
		`UPDATE topics SET is_active = FALSE, closed_at = NOW()
		 WHERE id = $1
		 RETURNING id, title, description, is_active, classifier_threshold, created_at, closed_at`,
		id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.IsActive, &t.ClassifierThreshold, &t.CreatedAt, &t.ClosedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("close topic: %w", err)
	}
	return t, nil
}