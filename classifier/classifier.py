import logging
import os
import re
import threading
from collections import OrderedDict

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
            result = self._classify_existing(req)
            if result is not None:
                return result

        return self._extract_new(req)

    def _classify_existing(self, req: ClassifyRequest) -> ClassifyResponse | None:
        result = self._pipeline(
            req.message,
            candidate_labels=req.existing_labels,
            multi_label=False,
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
        candidates = self._generate_extraction_labels(req.message)

        if not candidates:
            return ClassifyResponse(
                label=req.message[:50].title(),
                confidence=0.0,
                is_new=True,
            )

        try:
            result = self._pipeline(
                f"{req.message} [Topic: {req.topic}]",
                candidate_labels=candidates,
                multi_label=False,
                truncation=True,
                max_length=self.max_length,
            )

            top_label = result["labels"][0]
            top_score = result["scores"][0]

            return ClassifyResponse(
                label=top_label,
                confidence=top_score,
                is_new=True,
            )
        except Exception as e:
            logger.error(f"Extraction pipeline failed for message: {e}", exc_info=True)
            return ClassifyResponse(
                label=req.message[:50].title(),
                confidence=0.0,
                is_new=True,
            )

    def _generate_extraction_labels(self, message: str) -> list[str]:
        words = re.findall(r"\b[a-zA-Z]+\b", message.lower())
        words = [w for w in words if w not in STOP_WORDS and len(w) > 1]
        if not words:
            return []

        candidates = []
        for i in range(len(words)):
            for j in range(i + 2, min(i + 4, len(words) + 1)):
                phrase = " ".join(words[i:j])
                candidates.append(phrase)

        seen = set()
        unique = []
        for c in candidates:
            if c not in seen:
                seen.add(c)
                unique.append(c)

        return unique[:10] if unique else []