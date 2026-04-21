import logging
import os
from unittest.mock import MagicMock, patch

import pytest

from classifier import VoteClassifier
from schemas import ClassifyRequest, ClassifyResponse


@pytest.fixture
def mock_pipeline(mock_transformers_fixture):
    mock_pipe = MagicMock()
    mock_transformers_fixture.pipeline.return_value = mock_pipe
    yield mock_pipe
    mock_transformers_fixture.pipeline.reset_mock()


@pytest.fixture
def clf(mock_pipeline):
    return VoteClassifier()


class TestVoteClassifierInit:
    def test_default_config(self, mock_pipeline):
        clf = VoteClassifier()
        assert clf.model_name == "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli"
        assert clf.device == -1
        assert clf.max_length == 512
        assert clf.max_labels_per_topic == 100

    def test_env_config(self, mock_pipeline):
        with patch.dict(os.environ, {
            "CLASSIFIER_MODEL": "custom-model",
            "CLASSIFIER_DEVICE": "0",
            "CLASSIFIER_MAX_LENGTH": "256",
            "CLASSIFIER_MAX_LABELS_PER_TOPIC": "50",
        }):
            clf = VoteClassifier()
            assert clf.model_name == "custom-model"
            assert clf.device == 0
            assert clf.max_length == 256
            assert clf.max_labels_per_topic == 50


class TestClassifyExisting:
    def test_matches_existing_above_threshold(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["Pizza", "Sushi", "Burger"],
            "scores": [0.85, 0.10, 0.05],
        }

        req = ClassifyRequest(
            message="I love pizza",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi", "Burger"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.label == "Pizza"
        assert result.confidence == 0.85
        assert result.is_new is False
        assert result.all_scores == {"Pizza": 0.85, "Sushi": 0.10, "Burger": 0.05}

    def test_matches_existing_exactly_at_threshold(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["Sushi", "Pizza"],
            "scores": [0.5, 0.3],
        }

        req = ClassifyRequest(
            message="I like sushi",
            topic="Best Food",
            existing_labels=["Sushi", "Pizza"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.label == "Sushi"
        assert result.is_new is False

    def test_below_threshold_falls_to_extract(self, clf, mock_pipeline):
        mock_pipeline.side_effect = [
            {
                "labels": ["Pizza", "Sushi"],
                "scores": [0.2, 0.1],
            },
            {
                "labels": ["love pizza", "love"],
                "scores": [0.6, 0.2],
            },
        ]

        req = ClassifyRequest(
            message="I love pizza",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        call_count_before = mock_pipeline.call_count
        result = clf.classify(req)

        assert result.is_new is True
        assert mock_pipeline.call_count - call_count_before == 2

    def test_single_existing_label(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["Pizza"],
            "scores": [0.95],
        }

        req = ClassifyRequest(
            message="Give me pizza",
            topic="Food",
            existing_labels=["Pizza"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.label == "Pizza"
        assert result.confidence == 0.95
        assert result.is_new is False


class TestExtractNew:
    def test_no_existing_labels_extracts_new(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["beats tacos", "nothing beats"],
            "scores": [0.7, 0.2],
        }

        req = ClassifyRequest(
            message="Nothing beats tacos",
            topic="Best Food",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.all_scores is None

    def test_pipeline_error_falls_back(self, clf, mock_pipeline):
        mock_pipeline.side_effect = RuntimeError("model error")

        req = ClassifyRequest(
            message="Short msg",
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.confidence == 0.0
        assert result.label == "Short Msg"

    def test_pipeline_error_logs(self, clf, mock_pipeline, caplog):
        mock_pipeline.side_effect = RuntimeError("model error")

        req = ClassifyRequest(
            message="Short msg",
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        with caplog.at_level(logging.ERROR, logger="classifier"):
            clf.classify(req)

        assert any("Extraction pipeline failed" in r.message for r in caplog.records)

    def test_empty_message_no_candidates(self, clf, mock_pipeline):
        req = ClassifyRequest(
            message="!@#$%",
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.confidence == 0.0

    def test_long_message_truncation_fallback(self, clf, mock_pipeline):
        mock_pipeline.side_effect = RuntimeError("fail")

        long_msg = "A" * 200
        req = ClassifyRequest(
            message=long_msg,
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert len(result.label) <= 50


class TestLabelRegistry:
    def test_register_and_get_labels(self, clf, mock_pipeline):
        clf.register_label("topic-1", "Pizza")
        clf.register_label("topic-1", "Sushi")
        clf.register_label("topic-2", "Burger")

        assert clf.get_labels("topic-1") == ["Pizza", "Sushi"]
        assert clf.get_labels("topic-2") == ["Burger"]
        assert clf.get_labels("topic-3") == []

    def test_register_duplicate_label(self, clf, mock_pipeline):
        clf.register_label("topic-1", "Pizza")
        clf.register_label("topic-1", "Pizza")

        assert clf.get_labels("topic-1") == ["Pizza"]

    def test_max_labels_per_topic_eviction(self, mock_pipeline):
        with patch.dict(os.environ, {"CLASSIFIER_MAX_LABELS_PER_TOPIC": "3"}):
            clf = VoteClassifier()
            clf.register_label("topic-1", "A")
            clf.register_label("topic-1", "B")
            clf.register_label("topic-1", "C")
            clf.register_label("topic-1", "D")

            labels = clf.get_labels("topic-1")
            assert len(labels) == 3
            assert "A" not in labels

    def test_max_labels_per_topic_default(self, mock_pipeline):
        clf = VoteClassifier()
        assert clf.max_labels_per_topic == 100


class TestCandidateGeneration:
    def test_generates_candidates(self, clf, mock_pipeline):
        candidates = clf._generate_extraction_labels("I love pizza")
        assert len(candidates) > 0
        assert len(candidates) <= 10

    def test_filters_stop_words(self, clf, mock_pipeline):
        candidates = clf._generate_extraction_labels("I love pizza")
        for c in candidates:
            assert "i " not in c.lower() or c.startswith("i ")
            assert "[" not in c

    def test_no_unigrams(self, clf, mock_pipeline):
        candidates = clf._generate_extraction_labels("I love pizza today")
        for c in candidates:
            assert len(c.split()) >= 2

    def test_candidates_limited_to_ten(self, clf, mock_pipeline):
        long_msg = " ".join([f"word{i}" for i in range(20)])
        candidates = clf._generate_extraction_labels(long_msg)
        assert len(candidates) <= 10

    def test_special_characters_only_returns_empty(self, clf, mock_pipeline):
        candidates = clf._generate_extraction_labels("!@#$%")
        assert candidates == []