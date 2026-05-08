import logging
import os
import re
import threading
from collections import OrderedDict

from keybert import KeyBERT
from transformers import pipeline

from schemas import ClassifyRequest, ClassifyResponse

logger = logging.getLogger(__name__)

STOP_WORDS = frozenset({
    "a", "an", "the", "and", "or", "but", "is", "are", "was", "were", "be",
    "been", "being", "have", "has", "had", "do", "does", "did", "will", "would",
    "could", "should", "may", "might", "shall", "can", "need", "dare", "ought",
    "used", "to", "of", "in", "for", "on", "with", "at", "by", "from", "as",
    "into", "through", "during", "before", "after", "above", "below", "between",
    "out", "off", "over", "under", "again", "further", "then", "once", "here",
    "there", "when", "where", "why", "how", "all", "each", "every", "both",
    "few", "more", "most", "other", "some", "such", "no", "nor", "not", "only",
    "own", "same", "so", "than", "too", "very", "just", "because", "if", "that",
    "this", "these", "those", "it", "its", "i", "me", "my", "myself", "we",
    "our", "ours", "you", "your", "he", "him", "his", "she", "her", "they",
    "them", "their", "what", "which", "who", "whom",
})

DEFAULT_MAX_LABELS_PER_TOPIC = 100


class VoteClassifier:
    def __init__(self):
        self.model_name = os.getenv("CLASSIFIER_MODEL", "typeform/distilbert-base-uncased-mnli")
        self.device = int(os.getenv("CLASSIFIER_DEVICE", "-1"))
        self.max_length = int(os.getenv("CLASSIFIER_MAX_LENGTH", "512"))
        self.max_labels_per_topic = int(os.getenv("CLASSIFIER_MAX_LABELS_PER_TOPIC", str(DEFAULT_MAX_LABELS_PER_TOPIC)))
        self._pipeline = pipeline(
            "zero-shot-classification",
            model=self.model_name,
            device=self.device,
            truncation=True,
            max_length=self.max_length,
        )
        self._keybert = KeyBERT()
        self._label_registry: dict[str, OrderedDict[str, None]] = {}
        self._lock = threading.Lock()
        self._warmup()

    def _warmup(self):
        try:
            logger.info("Warming up model with dummy inference...")
            self._pipeline(
                "warmup",
                candidate_labels=["warmup"],
                multi_label=False,
                truncation=True,
                max_length=self.max_length,
            )
            # Warm up KeyBERT
            self._keybert.extract_keywords("warmup", top_n=1)
            logger.info("Model warmup complete.")
        except Exception as e:
            logger.warning(f"Model warmup failed: {e}")

    def register_label(self, topic_id: str, label: str):
        with self._lock:
            if topic_id not in self._label_registry:
                self._label_registry[topic_id] = OrderedDict()
            topic_labels = self._label_registry[topic_id]
            topic_labels[label] = None
            if len(topic_labels) > self.max_labels_per_topic:
                topic_labels.popitem(last=False)

    def get_labels(self, topic_id: str) -> list[str]:
        with self._lock:
            return list(self._label_registry.get(topic_id, {}).keys())

    def classify(self, req: ClassifyRequest) -> ClassifyResponse:
        if req.existing_labels:
            result = self._keyword_match(req)
            if result is not None:
                return result

            result = self._classify_existing(req)
            if result is not None:
                return result

        return self._extract_new(req)

    def _keyword_match(self, req: ClassifyRequest) -> ClassifyResponse | None:
        msg_words = set(re.findall(r"\b[a-zA-Z]+\b", req.message.lower()))

        best_label = None
        best_score: tuple[int, int, int] = (-1, 0, -1)
        best_label_word_count = 0
        best_overlap = 0

        for label in req.existing_labels:
            lwords = set(re.findall(r"\b[a-zA-Z]+\b", label.lower()))
            content_lwords = {w for w in lwords if w not in STOP_WORDS and len(w) > 1}

            if not content_lwords:
                continue

            overlap = sum(1 for w in (msg_words & lwords) if w not in STOP_WORDS and len(w) > 1)

            if overlap == 0:
                continue

            lcs_len = self._longest_common_substring_len(req.message.lower(), label.lower())

            score = (overlap, -len(content_lwords), lcs_len)

            if best_label is None or score > best_score or (score == best_score and label.lower() < best_label.lower()):
                best_score = score
                best_label = label
                best_label_word_count = len(content_lwords)
                best_overlap = overlap

        if best_label is None:
            return None

        confidence = best_overlap / best_label_word_count
        if confidence >= req.threshold:
            return ClassifyResponse(
                label=best_label,
                confidence=confidence,
                is_new=False,
                all_scores={best_label: confidence},
            )

        return None

    @staticmethod
    def _longest_common_substring_len(a: str, b: str) -> int:
        if not a or not b:
            return 0
        m, n = len(a), len(b)
        max_len = 0
        prev = [0] * (n + 1)
        for i in range(1, m + 1):
            curr = [0] * (n + 1)
            for j in range(1, n + 1):
                if a[i - 1] == b[j - 1]:
                    curr[j] = prev[j - 1] + 1
                    if curr[j] > max_len:
                        max_len = curr[j]
            prev = curr
        return max_len

    def _classify_existing(self, req: ClassifyRequest) -> ClassifyResponse | None:
        result = self._pipeline(
            req.message,
            candidate_labels=req.existing_labels,
            multi_label=True,
            truncation=True,
            max_length=self.max_length,
        )

        scores = dict(zip(result["labels"], result["scores"]))
        top_label = result["labels"][0]
        top_score = result["scores"][0]

        if top_score >= req.threshold:
            return ClassifyResponse(
                label=top_label,
                confidence=top_score,
                is_new=False,
                all_scores=scores,
            )

        return None

    def _extract_new(self, req: ClassifyRequest) -> ClassifyResponse:
        try:
            keywords = self._keybert.extract_keywords(
                req.message,
                keyphrase_ngram_range=(1, 2),  # 1-2 word labels
                stop_words="english",
                top_n=3,
                use_mmr=True,              # Maximal Marginal Relevance
                diversity=0.7,             # Diversity parameter for MMR
            )

            if keywords:
                top_keyword, top_score = keywords[0]
                return ClassifyResponse(
                    label=top_keyword,
                    confidence=top_score,
                    is_new=True,
                )
        except Exception as e:
            logger.error(f"KeyBERT extraction failed: {e}", exc_info=True)

        # Fallback
        return ClassifyResponse(
            label=req.message[:50].title(),
            confidence=0.0,
            is_new=True,
        )
