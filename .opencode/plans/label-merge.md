# Label Merge Feature Plan

## Goal

Allow the host/admin to merge labels in the leaderboard (e.g., "Tokyou" → "Tokyo") so that misspellings and variants are consolidated under a single label.

## UX Flow

1. Host sees the leaderboard with duplicate/similar labels.
2. Host selects multiple labels via checkboxes on the leaderboard.
3. Host clicks "Merge" and picks a target label (one of the selected ones, or a custom name).
4. Backend merges all votes from source labels into the target label.
5. Leaderboard updates in real-time via WebSocket (already handled by broadcast).

## Backend Changes

### 1. New API Endpoint

**`POST /api/votes/merge-labels`**

```json
// Request
{
  "topic_id": "uuid",
  "source_labels": ["Tokyou", "Tokio"],
  "target_label": "Tokyo"
}

// Response
{
  "topic_id": "uuid",
  "merged_labels": ["Tokyou", "Tokio"],
  "target_label": "Tokyo",
  "votes_affected": 15
}
```

- `target_label` can be one of the existing labels or a new custom string.
- All `source_labels` are merged INTO `target_label`.
- Returns 400 if topic doesn't exist or is not active/closed.
- Returns 400 if `source_labels` is empty or `target_label` is empty.

### 2. VoteTallyCache — new `MergeLabels` method

- Combines `VoteCount`, `TotalWeight`, and `LastVoteAt` from all source label tallies into the target label tally.
- Deletes source label entries from the `Labels` map.
- Appends a "merge record" to `batchBuf` so the DB is updated on next flush.

### 3. Repository — new `MergeLabels` method

```sql
UPDATE votes
SET classified_label = $1
WHERE topic_id = $2 AND classified_label = ANY($3)
```

Returns the count of affected rows.

### 4. Service layer

New `VoteService.MergeLabels` that:
1. Validates the topic exists.
2. Calls `voteRepo.MergeLabels` to update the DB.
3. Calls `tallyCache.MergeLabels` to update the in-memory cache.
4. Broadcasts updated leaderboard via WebSocket.
5. Returns the merge result.

### 5. No new database migration needed

No new tables required. We only UPDATE existing rows in `votes.classified_label`.

## Frontend Changes

### 1. API Client (`api/client.ts`)

Add:
```ts
mergeLabels(topicId: string, sourceLabels: string[], targetLabel: string): Promise<MergeResult>
```

### 2. Leaderboard Component (`components/Leaderboard.tsx`)

- Add a "Merge Mode" toggle button in the header.
- When active, each leaderboard entry shows a checkbox.
- Selected labels are tracked in state.
- A "Merge" action bar appears at the bottom with:
  - A dropdown to pick the target label (from selected labels or custom input).
  - A "Merge" button.
- On success, checkboxes clear and mode stays active for additional merges.

### 3. Host Page — no structural changes needed

The Leaderboard component handles everything internally.

## Classifier Impact

None. The classifier receives `existing_labels` from `tallyCache.GetLabels()`. After a merge:
- Source labels are removed from the cache → classifier won't see them as options.
- Target label exists in the cache → classifier will match new messages to it.

New incoming votes that would have been classified as "Tokyou" will instead either match "Tokyo" (if above threshold) or create a new label (if below threshold). This is correct behavior.

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/handler/vote.go` | Modify | Add `MergeLabels` handler |
| `backend/internal/service/vote.go` | Modify | Add `MergeLabels` service method |
| `backend/internal/service/vote_tally_cache.go` | Modify | Add `MergeLabels` cache method |
| `backend/internal/repository/vote.go` | Modify | Add `MergeLabels` DB method |
| `backend/internal/model/vote.go` | Modify | Add merge request/response types |
| `backend/cmd/server/main.go` | Modify | Register new route |
| `frontend/src/api/client.ts` | Modify | Add `mergeLabels` API call |
| `frontend/src/types/index.ts` | Modify | Add merge types |
| `frontend/src/components/Leaderboard.tsx` | Modify | Add merge mode UI |
