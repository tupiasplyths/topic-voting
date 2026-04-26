package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/repository"
)

const flushBatchSize = 1000

type LabelTally struct {
	TotalWeight int
	VoteCount   int
	LastVoteAt  time.Time
}

type TopicTally struct {
	mu         sync.RWMutex
	TopicTitle string
	Labels     map[string]*LabelTally
}

type VoteTallyCache struct {
	mu       sync.RWMutex
	tallies  map[uuid.UUID]*TopicTally
	repo     repository.VoteRepository
	batchBuf []*model.Vote
	flushMs  time.Duration
	quit     chan struct{}
	done     chan struct{}
}

func NewVoteTallyCache(ctx context.Context, repo repository.VoteRepository, flushMs time.Duration, topicRepo repository.TopicRepository) (*VoteTallyCache, error) {
	cache := &VoteTallyCache{
		tallies: make(map[uuid.UUID]*TopicTally),
		repo:    repo,
		flushMs: flushMs,
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}

	allTallies, err := repo.GetAllTallies(ctx)
	if err != nil {
		return nil, err
	}

	topics, err := topicRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	topicNames := make(map[uuid.UUID]string)
	for _, t := range topics {
		topicNames[t.ID] = t.Title
	}

	for topicID, entries := range allTallies {
		tt := &TopicTally{
			TopicTitle: topicNames[topicID],
			Labels:     make(map[string]*LabelTally),
		}
		for _, e := range entries {
			tt.Labels[e.Label] = &LabelTally{
				TotalWeight: e.TotalWeight,
				VoteCount:   e.VoteCount,
				LastVoteAt:  e.LastVoteAt,
			}
		}
		cache.tallies[topicID] = tt
	}

	return cache, nil
}

func (c *VoteTallyCache) Start() {
	go c.flushLoop()
}

func (c *VoteTallyCache) Stop() {
	close(c.quit)
	<-c.done
	c.flush()
}

func (c *VoteTallyCache) Increment(topicID uuid.UUID, topicTitle, label string, weight int, vote *model.Vote) {
	c.mu.Lock()
	tt, ok := c.tallies[topicID]
	if !ok {
		tt = &TopicTally{
			TopicTitle: topicTitle,
			Labels:     make(map[string]*LabelTally),
		}
		c.tallies[topicID] = tt
	}
	c.mu.Unlock()

	tt.mu.Lock()
	lt, ok := tt.Labels[label]
	if !ok {
		lt = &LabelTally{}
		tt.Labels[label] = lt
	}
	lt.TotalWeight += weight
	lt.VoteCount++
	lt.LastVoteAt = time.Now()
	tt.mu.Unlock()

	c.mu.Lock()
	c.batchBuf = append(c.batchBuf, vote)
	c.mu.Unlock()
}

func (c *VoteTallyCache) GetLeaderboard(topicID uuid.UUID) (*model.Leaderboard, error) {
	c.mu.RLock()
	tt, ok := c.tallies[topicID]
	c.mu.RUnlock()

	if !ok {
		return &model.Leaderboard{
			TopicID:   topicID,
			Entries:   []model.LeaderboardEntry{},
			UpdatedAt: time.Now(),
		}, nil
	}

	tt.mu.RLock()
	entries := make([]model.LeaderboardEntry, 0, len(tt.Labels))
	for label, lt := range tt.Labels {
		entries = append(entries, model.LeaderboardEntry{
			Label:       label,
			TotalWeight: lt.TotalWeight,
			VoteCount:   lt.VoteCount,
			LastVoteAt:  lt.LastVoteAt,
		})
	}
	tt.mu.RUnlock()

	return &model.Leaderboard{
		TopicID:   topicID,
		Topic:     tt.TopicTitle,
		Entries:   entries,
		UpdatedAt: time.Now(),
	}, nil
}

func (c *VoteTallyCache) GetLabels(topicID uuid.UUID) []string {
	c.mu.RLock()
	tt, ok := c.tallies[topicID]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	tt.mu.RLock()
	labels := make([]string, 0, len(tt.Labels))
	for label := range tt.Labels {
		labels = append(labels, label)
	}
	tt.mu.RUnlock()

	return labels
}

func (c *VoteTallyCache) MergeLabels(topicID uuid.UUID, sourceLabels []string, targetLabel string) {
	c.mu.RLock()
	tt, ok := c.tallies[topicID]
	c.mu.RUnlock()

	if !ok {
		return
	}

	tt.mu.Lock()
	defer tt.mu.Unlock()

	target, hasTarget := tt.Labels[targetLabel]
	if !hasTarget {
		target = &LabelTally{}
		tt.Labels[targetLabel] = target
	}

	for _, src := range sourceLabels {
		if src == targetLabel {
			continue
		}
		lt, ok := tt.Labels[src]
		if !ok {
			continue
		}
		target.TotalWeight += lt.TotalWeight
		target.VoteCount += lt.VoteCount
		if lt.LastVoteAt.After(target.LastVoteAt) {
			target.LastVoteAt = lt.LastVoteAt
		}
		delete(tt.Labels, src)
	}
}

func (c *VoteTallyCache) flushLoop() {
	defer close(c.done)
	ticker := time.NewTicker(c.flushMs)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.flush()
		case <-c.quit:
			return
		}
	}
}

func (c *VoteTallyCache) flush() {
	c.mu.Lock()
	if len(c.batchBuf) == 0 {
		c.mu.Unlock()
		return
	}
	batch := c.batchBuf
	c.batchBuf = make([]*model.Vote, 0, len(batch))
	c.mu.Unlock()

	var failed []*model.Vote
	for start := 0; start < len(batch); start += flushBatchSize {
		end := start + flushBatchSize
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]

		if err := c.repo.InsertBatch(context.Background(), chunk); err != nil {
			log.Printf("[tally-cache] flush error (chunk %d-%d): %v", start, end, err)
			failed = append(failed, chunk...)
		}
	}

	if len(failed) > 0 {
		c.mu.Lock()
		c.batchBuf = append(c.batchBuf, failed...)
		c.mu.Unlock()
	}
}
