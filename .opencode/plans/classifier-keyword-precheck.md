# Add Keyword Pre-Check Before Model Inference

## Problem
The zero-shot NLI model (`typeform/distilbert-base-uncased-mnli`) sometimes matches food messages to wrong labels (e.g., "pancakes are great" → Pizza at 1.000 confidence). It also runs on every message even for trivial matches, consuming CPU unnecessarily.

## Solution
Add a deterministic keyword word-overlap check **before** calling the model. The model only runs for messages that don't match any existing label by keyword, OR when the keyword match confidence falls below the caller's threshold.

## Algorithm

1. Extract words from the message: `.lower()` then `re.findall(r'\b[a-zA-Z]+\b', message)` → set `msg_words`
2. For each existing label, extract words the same way → set `lwords`
3. Compute non-stop-word overlap count:
   - `overlap = sum(1 for w in (msg_words & lwords) if w not in STOP_WORDS and len(w) > 1)`
4. Score each candidate label with the tuple:
   - Primary: `overlap` (more is better)
   - Secondary: `-len(lwords)` (shorter label preferred)
   - Tertiary: `lcs_length` = longest common substring length between `message.lower()` and `label.lower()` (character-level, longer is better)
   - Quaternary: alphabetical order of the label string
5. If no label has `overlap > 0`, return `None` → fall through to model
6. Take the best-scoring label. Confidence = `overlap / len(lwords)`
7. If `confidence >= req.threshold`, return `ClassifyResponse(label=..., confidence=..., is_new=False, all_scores={best_label: confidence})`
8. Otherwise return `None` → fall through to model

## New Flow

```
classify(req: ClassifyRequest):
  ┌─ req.existing_labels non-empty?
  │     YES → _keyword_match(req) → match?
  │     │         YES → return ClassifyResponse(label, confidence, is_new=False, all_scores={label: confidence})
  │     │         NO  → _classify_existing(req) → match?
  │     │                   YES → return ClassifyResponse(label, confidence=score, is_new=False, all_scores=scores)
  │     │                   NO  → _extract_new(req) → create new label
  │     NO  → _extract_new(req)
```

Key difference from model path: keyword matches return `all_scores` scoped to the single matched label (not all labels), since no model scores are available.

## Expected Accuracy (known test cases, threshold ≥ 0.5)

| Message | Labels | Before (model only) | After (keyword first, model fallback) |
|---|---|---|---|
| "I love pizza" | Pizza, Sushi, ... | Pizza ✓ | Pizza ✓ (keyword, no model call) |
| "pizza is the best" | Pizza, Sushi, ... | Pizza ✓ | Pizza ✓ (keyword, no model call) |
| "voting for sushi" | Sushi, ... | Sushi ✓ | Sushi ✓ (keyword, no model call) |
| "nothing beats tacos" | Tacos, ... | Tacos ✓ | Tacos ✓ (keyword, no model call) |
| **"pancakes are great"** | Pizza, Sushi, ... | **Pizza ✗** | None → _classify_existing (model, below threshold) → _extract_new (model) → new label ✓ |
| "lets go" | Pizza, ... | NEW ✓ | keyword None → _classify_existing (model, below threshold) → _extract_new (model) → NEW ✓ |
| "bruh" | Pizza, ... | NEW ✓ | same flow → NEW ✓ |
| "this is great" | Pizza, ... | NEW ✓ | same flow → NEW ✓ |
| "morning coffee" | Pizza, ... | NEW ✓ | same flow → NEW ✓ |
| "I think sushi is the best" | sushi deserves win, choose sushi, sushi clearly, ramen best | sushi clearly ✓ | sushi ✓ (keyword, no model call; alphabetical tiebreak among {overlap=1,len=2,lcs=5} labels) |
| "choose sushi!" | choose sushi, ... | choose sushi ✓ | choose sushi ✓ (keyword, no model call) |
| "ramen is life" | ramen best, gotta ramen | ramen best ✓ | ramen ✓ (keyword, no model call; alphabetical tiebreak among {overlap=1,len=2,lcs=3} labels) |

**Note**: For tied cases (equal overlap, equal word count, equal LCS), alphabetical order breaks the tie. Both tied labels are semantically valid for the input message.

## Changes

### `classifier/classifier.py`
- Add `_keyword_match(self, req: ClassifyRequest) → ClassifyResponse | None`:
  - Extracts words from `req.message` (lowercased, alpha only)
  - Extracts words from each label in `req.existing_labels`
  - Scores labels by `(overlap, -len(lwords), lcs_len, alphabetical_name)`
  - Returns `ClassifyResponse(label, confidence=overlap/len(lwords), is_new=False, all_scores={label: confidence})` if `confidence >= req.threshold`
  - Returns `None` otherwise
- Update `classify()` to call `_keyword_match` before `_classify_existing`
- `label_word_count` = `len(lwords)` (unique non-stop words in label after filtering)
- No config or backend changes needed

### `classifier/tests/test_classifier.py`
- Add `TestKeywordMatch` class:
  - `test_exact_word_match` → returns label with confidence 1.0
  - `test_partial_word_match` → returns label with proportional confidence
  - `test_no_overlap` → returns None
  - `test_overlap_below_threshold` → returns None when confidence < req.threshold
  - `test_stop_words_ignored` → stop words don't count for overlap
  - `test_tiebreak_shorter_label` → shorter label wins on equal overlap
  - `test_tiebreak_alphabetical` → alphabetical order breaks ties at same overlap + same length + same LCS
  - `test_compound_labels` → multi-word label matching
  - `test_all_scores_present` → keyword match returns `all_scores` with single entry
  - `test_message_with_no_content_words` → returns None

## Impact on model calls
- **No model call**: Messages whose words directly overlap with existing labels AND confidence passes threshold → ~80% of traffic (estimate)
- **Still calls model**: Messages with no keyword overlap, or overlap below threshold → _classify_existing then possibly _extract_new

## Edge Cases Handled
- **Labels consisting entirely of stop words** → `lwords` would be empty set, `len(lwords) = 0`. Guard against division by zero: if `len(lwords) == 0`, skip that label.
- **Message with no content words** → `msg_words` empty set, overlap = 0 for all labels, return None.
- **Labels with duplicate words** → sets deduplicate, so "pizza pizza" becomes `{"pizza"}` with `len=1`.
- **Labels with numbers/special chars** → same treatment as `_generate_extraction_labels` (`re.findall(r'\b[a-zA-Z]+\b', ...)`), consistent with existing behavior.
