# Chat-Voted vs Donation-Voted Topics — Revised Plan

## Behavioral Rules

| Mode | Non-donation weight | Bits donation weight | Direct donation weight |
|---|---|---|---|
| **Chat** (default) | `1` | `1 + bits/100` (existing) | `1 + amount` |
| **Donation** | `0` (classified & stored, no leaderboard impact) | `bits / 100.0` (USD) | `amount × exchange_rate(currency)` |

- All messages are always classified, stored, and shown in chat UI regardless of mode
- The mode **only** affects weight computation
- Bits are a type of donation (100 bits = $1.00 in donation mode)
- Exchange rates: static JSON map via `EXCHANGE_RATES` env var

---

## Phase 1: Database Migration

File: `backend/migrations/002_voting_mode_and_currency.sql`

```sql
DROP MATERIALIZED VIEW IF EXISTS vote_tallies;
DROP INDEX IF EXISTS idx_vote_tallies_topic_label;

ALTER TABLE topics ADD COLUMN voting_mode VARCHAR(20) NOT NULL DEFAULT 'chat';
ALTER TABLE topics ADD CONSTRAINT chk_voting_mode CHECK (voting_mode IN ('chat', 'donation'));

ALTER TABLE votes ALTER COLUMN weight TYPE DECIMAL(12,2);

ALTER TABLE votes ADD COLUMN donation_amount DECIMAL(12,2) NOT NULL DEFAULT 0;
ALTER TABLE votes ADD COLUMN donation_currency VARCHAR(3) NOT NULL DEFAULT '';

CREATE MATERIALIZED VIEW vote_tallies AS
SELECT
    topic_id,
    classified_label,
    SUM(weight) AS total_weight,
    COUNT(*)    AS vote_count,
    MAX(created_at) AS last_vote_at
FROM votes
GROUP BY topic_id, classified_label
ORDER BY topic_id, total_weight DESC;

CREATE UNIQUE INDEX idx_vote_tallies_topic_label ON vote_tallies (topic_id, classified_label);
```

---

## Phase 2: Backend Models

### `model/topic.go`

- Add `VotingMode string \`json:"voting_mode"\`` to `Topic` struct
- Add `VotingMode *string \`json:"voting_mode,omitempty"\`` to `CreateTopicRequest` struct (omit means `"chat"` default)

### `model/vote.go`

- Change `Weight int` → `Weight float64` in `Vote` struct
- Add `DonationAmount float64 \`json:"donation_amount"\``
- Add `DonationCurrency string \`json:"donation_currency"\``
- Add `DonationAmount float64 \`json:"donation_amount"\`` to `SubmitVoteRequest`
- Add `DonationCurrency string \`json:"donation_currency"\`` to `SubmitVoteRequest`
- Change `TotalWeight int` → `TotalWeight float64` in `LeaderboardEntry`
- Add `VotingMode string \`json:"voting_mode"\`` to `Leaderboard`

---

## Phase 3: Exchange Rate Service

New file: `backend/internal/service/exchange_rates.go`

```go
type Exchanger interface {
    ToUSD(amount float64, currency string) float64
}

type ExchangeRateService struct {
    rates map[string]float64
}

func NewExchangeRateService(ratesJSON string) (*ExchangeRateService, error)
func (s *ExchangeRateService) ToUSD(amount float64, currency string) float64
```

- Default rates: `{"USD":1.0,"CAD":0.74,"EUR":1.08,"GBP":1.27,"AUD":0.65,"BRL":0.19}`
- Parse from `EXCHANGE_RATES` env var, fall back to defaults if empty
- Unknown currency → rate = 1.0 (treat as USD)

---

## Phase 4: Backend Repository

### `repository/topic.go`

- Add `voting_mode` to all queries:
  - `Create` / `CreateWithTx`: INSERT includes `voting_mode`
  - `List`: SELECT includes `voting_mode`
  - `GetActive`, `GetByID`, `Close`: SELECT includes `voting_mode`
- Update all Scan calls to read `voting_mode`

### `repository/vote.go`

- Add `donation_amount`, `donation_currency` to `InsertBatch` INSERT query
- Change scan targets for `total_weight` from `int` to `float64`

---

## Phase 5: Backend Service

### `service/vote_processor.go`

- Add `VotingMode string`, `DonationAmount float64`, `DonationCurrency string` to `PendingVote` struct
- Add `Exchanger` interface field to `VoteProcessor`, inject via `NewVoteProcessor`
- Replace `computeWeight(isDonation bool, bitsAmount int) int` with:

```go
func computeWeight(pv *PendingVote, exchanger Exchanger) float64 {
    switch pv.VotingMode {
    case "donation":
        if !pv.IsDonation {
            return 0
        }
        if pv.DonationAmount > 0 {
            return exchanger.ToUSD(pv.DonationAmount, pv.DonationCurrency)
        }
        return float64(pv.BitsAmount) / 100.0
    default: // "chat"
        if !pv.IsDonation {
            return 1.0
        }
        if pv.DonationAmount > 0 {
            return 1.0 + pv.DonationAmount
        }
        return float64(1 + pv.BitsAmount/100)
    }
}
```

- Update `worker()` to use new `computeWeight`
- Change `WSClassifiedData.Weight` from `int` to `float64`

### `service/vote.go`

- `SubmitVote`: pass `topic.VotingMode`, `req.DonationAmount`, `req.DonationCurrency` into `PendingVote`
- Update or remove the `computeWeight` call at line 64 (weight is now computed in worker; the return value can be a placeholder or removed)

### `service/vote_tally_cache.go`

- Change `LabelTally.TotalWeight` from `int` to `float64`
- Change `Increment()` weight parameter from `int` to `float64`
- Change `lt.TotalWeight += weight` accordingly
- Pass `voting_mode` through into `Leaderboard` struct in `GetLeaderboard()`

### `service/topic.go`

- `Create()`: handle `VotingMode` from request, default to `"chat"`

---

## Phase 6: Backend Wiring

### `config/config.go`

- Add `ExchangeRates string` field (raw JSON string from `EXCHANGE_RATES` env var)

### `cmd/server/main.go`

- Parse `EXCHANGE_RATES` env var via `cfg.ExchangeRates`
- Instantiate `service.NewExchangeRateService(cfg.ExchangeRates)`
- Pass exchange rate service to `NewVoteProcessor`

### `handler/websocket.go`

- Update `handleChatMessage` inline struct to unmarshal `donation_amount` (`float64`) and `donation_currency` (`string`)
- Pass new fields into `model.SubmitVoteRequest`

---

## Phase 7: Frontend Types

### `src/types/index.ts`

- Add `voting_mode: 'chat' | 'donation'` to `Topic` interface
- Add `voting_mode?: 'chat' | 'donation'` to `CreateTopicRequest` interface
- Add `voting_mode: 'chat' | 'donation'` to `Leaderboard` interface

### `src/types/chat.ts`

- Add `donation_amount: number` to `ChatMessage` interface
- Add `donation_currency: string` to `ChatMessage` interface

---

## Phase 8: Frontend TopicManager

File: `src/components/TopicManager.tsx`

- Add radio buttons for voting mode selection:
  - **Chat-voted** (default): every message = +1 vote, donations add bits bonus
  - **Donation-voted**: only donations count, weighted by USD value
- Include `voting_mode` in `CreateTopicRequest`
- Show `(Chat-voted)` / `(Donation-voted)` badge on the active topic card
- Show voting mode badge on each topic in the All Topics list

---

## Phase 9: Frontend MockChat

### `src/utils/mockChat.ts`

- `generateDonationMessage` returns `{message, item, bits, donation_amount, donation_currency}`
- Random `donation_amount`: $5–$500 (step $5)
- Random `donation_currency`: USD, CAD, EUR, GBP

### `src/components/MockChat.tsx`

- Pass `donation_amount` and `donation_currency` through the `ChatMessage` object
- Include new fields in the WebSocket JSON payload sent to backend
- Show `$X.XX` for donation amounts alongside bits in donation messages

---

## Phase 10: Frontend Display Updates

### `Leaderboard.tsx`

- Show voting mode badge in header: `Chat-voted` / `Donation-voted`
- Format leaderboard weights:
  - Chat mode: `X pts`
  - Donation mode: `$X.XX`
- Accept `voting_mode` from `Leaderboard` data

### `VoteBarChart.tsx`

- Accept `voting_mode` prop
- Format tooltip: `X pts` in chat mode, `$X.XX` in donation mode

### `DisplayOverlay.tsx`

- Accept `voting_mode` prop
- Format points display: `X pts` vs `$X.XX`
- Replace `hasDonations` heuristic (`vote_count !== total_weight`) with explicit data or remove it (the heuristic is unreliable with float weights)

### `HostPage.tsx` / `DisplayPage.tsx`

- Thread `voting_mode` from WS `Leaderboard` data to children (`VoteBarChart`, `DisplayOverlay`)

---

## Key Design Decisions

| Decision | Choice |
|---|---|
| Non-donations in donation mode | Weight = 0 (still classified, stored, and shown in chat) |
| Weight formula (chat mode) | `1` for messages, `1 + bits/100` for bits, `1 + amount` for direct donations |
| Weight formula (donation mode) | `bits/100` for bits, `amount × exchange_rate` for direct donations |
| Bits-to-USD conversion | 100 bits = $1.00 |
| Exchange rates | Configurable via `EXCHANGE_RATES` env var (no external API dependency) |
| Fallback for unknown currencies | Treat as USD (rate = 1.0) |
| Weight type | `float64` in Go, `DECIMAL(12,2)` in DB, `number` in frontend |
| `bits_amount` field | Kept for backward compatibility; `donation_amount`/`donation_currency` take precedence when present |

---

## Files Changed Summary

| File | Change type |
|---|---|
| `backend/migrations/002_voting_mode_and_currency.sql` | **New** |
| `backend/internal/model/topic.go` | Add fields |
| `backend/internal/model/vote.go` | Change types + add fields |
| `backend/internal/service/exchange_rates.go` | **New** |
| `backend/internal/config/config.go` | Add field |
| `backend/internal/repository/topic.go` | Add column to queries |
| `backend/internal/repository/vote.go` | Add columns, change types |
| `backend/internal/service/vote_processor.go` | New computeWeight + add field |
| `backend/internal/service/vote.go` | Pass mode + donation fields |
| `backend/internal/service/vote_tally_cache.go` | Change weight to float64 |
| `backend/internal/service/topic.go` | Handle VotingMode |
| `backend/internal/handler/websocket.go` | Parse donation fields |
| `backend/cmd/server/main.go` | Wire exchange rate service |
| `frontend/src/types/index.ts` | Add voting_mode fields |
| `frontend/src/types/chat.ts` | Add donation fields |
| `frontend/src/components/TopicManager.tsx` | Voting mode selector + badges |
| `frontend/src/components/MockChat.tsx` | Pass donation fields |
| `frontend/src/utils/mockChat.ts` | Generate donation amounts |
| `frontend/src/components/Leaderboard.tsx` | Badge + conditional formatting |
| `frontend/src/components/VoteBarChart.tsx` | Accept voting_mode prop |
| `frontend/src/components/DisplayOverlay.tsx` | Accept voting_mode prop |
| `frontend/src/pages/HostPage.tsx` | Thread voting_mode |
| `frontend/src/pages/DisplayPage.tsx` | Thread voting_mode |
