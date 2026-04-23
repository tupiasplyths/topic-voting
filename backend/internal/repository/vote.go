package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/topic-voting/backend/internal/model"
)

type VoteRepository interface {
	InsertBatch(ctx context.Context, votes []*model.Vote) error
	GetTalliesByTopic(ctx context.Context, topicID uuid.UUID) ([]model.LeaderboardEntry, error)
	GetAllTallies(ctx context.Context) (map[uuid.UUID][]model.LeaderboardEntry, error)
}

type voteRepo struct {
	pool *pgxpool.Pool
}

func NewVoteRepository(pool *pgxpool.Pool) VoteRepository {
	return &voteRepo{pool: pool}
}

func (r *voteRepo) InsertBatch(ctx context.Context, votes []*model.Vote) error {
	if len(votes) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, v := range votes {
		batch.Queue(
			`INSERT INTO votes (id, topic_id, username, raw_message, classified_label, confidence, weight, is_donation, bits_amount, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			v.ID, v.TopicID, v.Username, v.RawMessage, v.ClassifiedLabel, v.Confidence, v.Weight, v.IsDonation, v.BitsAmount, v.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range votes {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert vote: %w", err)
		}
	}

	return nil
}

func (r *voteRepo) GetTalliesByTopic(ctx context.Context, topicID uuid.UUID) ([]model.LeaderboardEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT classified_label, SUM(weight) AS total_weight, COUNT(*) AS vote_count, MAX(created_at) AS last_vote_at
		 FROM votes WHERE topic_id = $1
		 GROUP BY classified_label
		 ORDER BY total_weight DESC`, topicID)
	if err != nil {
		return nil, fmt.Errorf("get tallies: %w", err)
	}
	defer rows.Close()

	entries := make([]model.LeaderboardEntry, 0)
	for rows.Next() {
		var e model.LeaderboardEntry
		if err := rows.Scan(&e.Label, &e.TotalWeight, &e.VoteCount, &e.LastVoteAt); err != nil {
			return nil, fmt.Errorf("scan tally: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *voteRepo) GetAllTallies(ctx context.Context) (map[uuid.UUID][]model.LeaderboardEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT topic_id, classified_label, SUM(weight) AS total_weight, COUNT(*) AS vote_count, MAX(created_at) AS last_vote_at
		 FROM votes GROUP BY topic_id, classified_label
		 ORDER BY topic_id, total_weight DESC`)
	if err != nil {
		return nil, fmt.Errorf("get all tallies: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]model.LeaderboardEntry)
	for rows.Next() {
		var topicID uuid.UUID
		var e model.LeaderboardEntry
		if err := rows.Scan(&topicID, &e.Label, &e.TotalWeight, &e.VoteCount, &e.LastVoteAt); err != nil {
			return nil, fmt.Errorf("scan tally: %w", err)
		}
		result[topicID] = append(result[topicID], e)
	}
	return result, rows.Err()
}