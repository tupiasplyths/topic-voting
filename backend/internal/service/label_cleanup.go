package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/topic-voting/backend/internal/model"
	"github.com/topic-voting/backend/internal/repository"
)

type LabelCleanupService struct {
	classifier ClassifierClient
	voteRepo   repository.VoteRepository
	topicRepo  repository.TopicRepository
	tallyCache *VoteTallyCache
	wsHub      WSBroadcaster
	interval   time.Duration
	threshold  float64
	similarity float64
	quit       chan struct{}
	wg         sync.WaitGroup
}

func NewLabelCleanupService(
	classifier ClassifierClient,
	voteRepo repository.VoteRepository,
	topicRepo repository.TopicRepository,
	tallyCache *VoteTallyCache,
	wsHub WSBroadcaster,
	interval time.Duration,
	threshold float64,
	similarity float64,
) *LabelCleanupService {
	return &LabelCleanupService{
		classifier: classifier,
		voteRepo:   voteRepo,
		topicRepo:  topicRepo,
		tallyCache: tallyCache,
		wsHub:      wsHub,
		interval:   interval,
		threshold:  threshold,
		similarity: similarity,
		quit:       make(chan struct{}),
	}
}

func (s *LabelCleanupService) Start() {
	s.wg.Add(1)
	go s.loop()
}

func (s *LabelCleanupService) Stop() {
	close(s.quit)
	s.wg.Wait()
}

func (s *LabelCleanupService) loop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runCleanup()
		case <-s.quit:
			return
		}
	}
}

func (s *LabelCleanupService) runCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic, err := s.topicRepo.GetActive(ctx)
	if err != nil {
		log.Printf("[label-cleanup] error fetching active topic: %v", err)
		return
	}
	if topic == nil {
		return
	}

	s.cleanupTopic(ctx, topic.ID, topic.Title)
}

func (s *LabelCleanupService) cleanupTopic(ctx context.Context, topicID uuid.UUID, topicTitle string) {
	// Note: There is an inherent TOCTOU race — the leaderboard is read here, but
	// new labels may be added between this read and the subsequent merge operations.
	// This is acceptable because cleanup is periodic and best-effort; any stale
	// labels will be caught on the next cycle. The cache is always consistent with
	// itself due to MergeLabels being applied atomically within the cache.
	lb, err := s.tallyCache.GetLeaderboard(topicID)
	if err != nil {
		log.Printf("[label-cleanup] error fetching leaderboard for %s: %v", topicID, err)
		return
	}

	entries := lb.Entries
	if len(entries) == 0 {
		return
	}

	labels := make([]string, 0, len(entries))
	voteCounts := make(map[string]int, len(entries))
	for _, e := range entries {
		if e.Label == model.OffTopicSentinel {
			continue
		}
		labels = append(labels, e.Label)
		voteCounts[e.Label] = e.VoteCount
	}
	if len(labels) == 0 {
		return
	}

	changes := 0
	totalBefore := len(labels)

	classifyCtx, classifyCancel := context.WithTimeout(ctx, 10*time.Second)
	// Use a descriptive prompt so the zero-shot classifier can meaningfully
	// score each label's relevance to the topic (e.g. "This topic is about: Favorite Foods").
	classifyMessage := fmt.Sprintf("This topic is about: %s", topicTitle)
	result, err := s.classifier.Classify(classifyCtx, classifyMessage, topicTitle, labels, 0.0)
	classifyCancel()

	if err != nil {
		log.Printf("[label-cleanup] classifier error for topic %s: %v, will retry later", topicID, err)
		return
	}
	if result.AllScores == nil {
		log.Printf("[label-cleanup] classifier returned no scores for topic %s, will retry later", topicID)
		return
	}

	offTopic := s.findOffTopic(result.AllScores)
	if len(offTopic) > 0 {
		s.mergeLabels(ctx, topicID, offTopic, model.OffTopicSentinel)
		for _, l := range offTopic {
			log.Printf("[label-cleanup]   off-topic: %q (relevance=%.2f) → merged into %s",
				l, result.AllScores[l], model.OffTopicSentinel)
		}
		changes += len(offTopic)
	}

	remaining := exclude(labels, offTopic)
	groups := s.findSimilarGroups(remaining, voteCounts)
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		canonical := s.pickCanonical(g, voteCounts)
		sources := make([]string, 0, len(g)-1)
		for _, l := range g {
			if l != canonical {
				sources = append(sources, l)
			}
		}
		if len(sources) == 0 {
			continue
		}
		s.mergeLabels(ctx, topicID, sources, canonical)
		log.Printf("[label-cleanup]   similar group: %v → merged into %q (%d votes)",
			jsonLabels(sources), canonical, voteCounts[canonical])
		changes += len(sources)
	}

	if changes > 0 {
		log.Printf("[label-cleanup] topic=%q id=%s done: %d labels removed/merged (was %d, now ~%d)",
			topicTitle, topicID, changes, totalBefore, totalBefore-changes)
	}
}

func (s *LabelCleanupService) mergeLabels(ctx context.Context, topicID uuid.UUID, sources []string, target string) {
	_, merr := s.voteRepo.MergeLabels(ctx, topicID, sources, target)
	if merr != nil {
		log.Printf("[label-cleanup]   db merge error for %s: %v", topicID, merr)
	}
	s.tallyCache.MergeLabels(topicID, sources, target)
	go s.wsHub.BroadcastLeaderboard(topicID)
}

func (s *LabelCleanupService) findOffTopic(scores map[string]float64) []string {
	var off []string
	for label, score := range scores {
		if score < s.threshold {
			off = append(off, label)
		}
	}
	return off
}

// findSimilarGroups uses O(n²) pair-wise comparison to group labels whose
// LevenshteinRatio meets the similarity threshold. Groups are computed via
// connected components (BFS) on the similarity adjacency graph. Acceptable
// because label count per topic is typically well under 100.
func (s *LabelCleanupService) findSimilarGroups(labels []string, voteCounts map[string]int) [][]string {
	norm := make([]string, len(labels))
	for i, l := range labels {
		norm[i] = strings.ToLower(strings.TrimSpace(l))
	}

	adj := make(map[string][]string)
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			if levenshteinRatio(norm[i], norm[j]) >= s.similarity {
				adj[labels[i]] = append(adj[labels[i]], labels[j])
				adj[labels[j]] = append(adj[labels[j]], labels[i])
			}
		}
	}

	visited := make(map[string]bool)
	var groups [][]string

	for _, label := range labels {
		if visited[label] {
			continue
		}
		group := []string{}
		queue := []string{label}
		visited[label] = true
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			group = append(group, curr)
			for _, neighbor := range adj[curr] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		if len(group) > 1 {
			groups = append(groups, group)
		}
	}

	return groups
}

func (s *LabelCleanupService) pickCanonical(labels []string, voteCounts map[string]int) string {
	if len(labels) == 0 {
		return ""
	}
	best := labels[0]
	bestScore := voteCounts[best]
	for _, l := range labels[1:] {
		vc := voteCounts[l]
		if vc > bestScore || (vc == bestScore && len(l) < len(best)) {
			best = l
			bestScore = vc
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if len(a) < len(b) {
		return levenshtein(b, a)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(b)]
}

func levenshteinRatio(a, b string) float64 {
	if a == b {
		return 1.0
	}
	dist := levenshtein(a, b)
	maxLen := dist
	if len(a) > maxLen {
		maxLen = len(a)
	}
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

func exclude(all, remove []string) []string {
	rm := make(map[string]bool, len(remove))
	for _, r := range remove {
		rm[r] = true
	}
	j := 0
	for _, v := range all {
		if !rm[v] {
			all[j] = v
			j++
		}
	}
	return all[:j]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func jsonLabels(labels []string) string {
	b, _ := json.Marshal(labels)
	return string(b)
}
