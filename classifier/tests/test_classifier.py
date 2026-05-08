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
def mock_keybert(mock_keybert_fixture):
    mock_kw_model = MagicMock()
    mock_keybert_fixture.KeyBERT.return_value = mock_kw_model
    yield mock_kw_model


@pytest.fixture
def clf(mock_pipeline, mock_keybert):
    return VoteClassifier()


class TestVoteClassifierInit:
    def test_default_config(self, mock_pipeline):
        clf = VoteClassifier()
        assert clf.model_name == "typeform/distilbert-base-uncased-mnli"
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
            message="I really enjoy Italian flatbread with cheese",
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
            message="I like raw fish on seasoned rice",
            topic="Best Food",
            existing_labels=["Sushi", "Pizza"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.label == "Sushi"
        assert result.confidence == 0.5
        assert result.is_new is False

    def test_below_threshold_falls_to_extract(self, clf, mock_pipeline, mock_keybert):
        mock_pipeline.return_value = {
            "labels": ["Pizza", "Sushi"],
            "scores": [0.2, 0.1],
        }
        mock_keybert.extract_keywords.return_value = [
            ("great stuff", 0.6),
            ("stuff", 0.4),
            ("great", 0.35),
        ]

        req = ClassifyRequest(
            message="this is great stuff",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        call_count_before = mock_pipeline.call_count
        result = clf.classify(req)

        assert result.is_new is True
        assert result.label == "great stuff"
        assert mock_pipeline.call_count - call_count_before == 1

    def test_single_existing_label(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["Pizza"],
            "scores": [0.95],
        }

        req = ClassifyRequest(
            message="I really enjoy Italian flatbread",
            topic="Food",
            existing_labels=["Pizza"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.label == "Pizza"
        assert result.confidence == 0.95
        assert result.is_new is False

    def test_keyword_miss_falls_to_model(self, clf, mock_pipeline):
        mock_pipeline.return_value = {
            "labels": ["Pizza", "Sushi"],
            "scores": [0.85, 0.10],
        }
        call_count_after_init = mock_pipeline.call_count

        req = ClassifyRequest(
            message="pancakes are great",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert mock_pipeline.call_count == call_count_after_init + 1
        assert result.label == "Pizza"
        assert result.confidence == 0.85


class TestExtractNew:
    def test_extracts_keyword(self, clf, mock_keybert):
        mock_keybert.extract_keywords.return_value = [
            ("tacos", 0.82),
            ("beats tacos", 0.65),
            ("nothing beats", 0.51),
        ]

        req = ClassifyRequest(
            message="Nothing beats tacos",
            topic="Best Food",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.label == "tacos"
        assert result.confidence == 0.82
        assert result.all_scores is None

    def test_extracts_bigram(self, clf, mock_keybert):
        mock_keybert.extract_keywords.return_value = [
            ("new york", 0.78),
            ("york", 0.55),
            ("new", 0.40),
        ]

        req = ClassifyRequest(
            message="New York never sleeps",
            topic="Cities",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.label == "new york"
        assert result.confidence == 0.78

    def test_no_keywords_fallback(self, clf, mock_keybert):
        mock_keybert.extract_keywords.return_value = []

        req = ClassifyRequest(
            message="!@#$%",
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert result.is_new is True
        assert result.confidence == 0.0

    def test_keybert_error_fallback(self, clf, mock_keybert):
        mock_keybert.extract_keywords.side_effect = RuntimeError("model error")

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

    def test_keybert_error_logs(self, clf, mock_keybert, caplog):
        mock_keybert.extract_keywords.side_effect = RuntimeError("model error")

        req = ClassifyRequest(
            message="Short msg",
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        with caplog.at_level(logging.ERROR, logger="classifier"):
            clf.classify(req)

        assert any("KeyBERT extraction failed" in r.message for r in caplog.records)

    def test_long_message_fallback(self, clf, mock_keybert):
        mock_keybert.extract_keywords.side_effect = RuntimeError("fail")

        long_msg = "A" * 200
        req = ClassifyRequest(
            message=long_msg,
            topic="Test",
            existing_labels=[],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert len(result.label) <= 50

    def test_below_threshold_falls_to_extract(self, clf, mock_pipeline, mock_keybert):
        mock_pipeline.return_value = {
            "labels": ["Pizza", "Sushi"],
            "scores": [0.2, 0.1],
        }
        mock_keybert.extract_keywords.return_value = [
            ("great stuff", 0.6),
            ("stuff", 0.4),
            ("great", 0.35),
        ]

        req = ClassifyRequest(
            message="this is great stuff",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        call_count_before = mock_pipeline.call_count
        result = clf.classify(req)

        assert result.is_new is True
        assert result.label == "great stuff"
        assert mock_pipeline.call_count - call_count_before == 1

    def test_extract_uses_mmr(self, clf, mock_keybert):
        mock_keybert.extract_keywords.return_value = [("pizza", 0.9)]

        req = ClassifyRequest(
            message="I love pizza",
            topic="Food",
            existing_labels=[],
            threshold=0.5,
        )
        clf.classify(req)

        # Warmup calls extract_keywords once, so check the last call
        mock_keybert.extract_keywords.assert_called_with(
            "I love pizza",
            keyphrase_ngram_range=(1, 2),
            stop_words="english",
            top_n=3,
            use_mmr=True,
            diversity=0.7,
        )


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


class TestKeywordMatch:
    def test_exact_word_match(self, clf):
        req = ClassifyRequest(
            message="pizza",
            topic="Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.label == "Pizza"
        assert result.confidence == 1.0
        assert result.is_new is False

    def test_partial_word_match(self, clf):
        req = ClassifyRequest(
            message="voting for sushi today",
            topic="Food",
            existing_labels=["sushi deserves win", "choose sushi", "sushi clearly"],
            threshold=0.3,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert "sushi" in result.label.lower()
        assert result.confidence > 0

    def test_no_overlap(self, clf):
        req = ClassifyRequest(
            message="pancakes are great",
            topic="Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is None

    def test_overlap_below_threshold(self, clf):
        req = ClassifyRequest(
            message="I love sushi and pizza",
            topic="Food",
            existing_labels=["Pizza Margherita Special"],
            threshold=0.8,
        )
        result = clf._keyword_match(req)

        assert result is None

    def test_stop_words_ignored(self, clf):
        req = ClassifyRequest(
            message="the pizza is the best",
            topic="Food",
            existing_labels=["Pizza"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.label == "Pizza"
        assert result.confidence == 1.0

    def test_tiebreak_shorter_label(self, clf):
        req = ClassifyRequest(
            message="I love ramen",
            topic="Food",
            existing_labels=["Ramen Special", "Ramen"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.label == "Ramen"

    def test_tiebreak_alphabetical(self, clf):
        req = ClassifyRequest(
            message="I think sushi is the best",
            topic="Food",
            existing_labels=["sushi clearly", "sushi deserves"],
            threshold=0.3,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.label == "sushi clearly"

    def test_compound_labels(self, clf):
        req = ClassifyRequest(
            message="choose sushi for dinner",
            topic="Food",
            existing_labels=["choose sushi", "go with pizza"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.label == "choose sushi"

    def test_all_scores_present(self, clf):
        req = ClassifyRequest(
            message="pizza",
            topic="Food",
            existing_labels=["Pizza"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is not None
        assert result.all_scores is not None
        assert len(result.all_scores) == 1
        assert "Pizza" in result.all_scores

    def test_message_with_no_content_words(self, clf):
        req = ClassifyRequest(
            message="I am the best",
            topic="Food",
            existing_labels=["Pizza", "Sushi"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is None

    def test_label_all_stop_words_skipped(self, clf):
        req = ClassifyRequest(
            message="the and or is",
            topic="Food",
            existing_labels=["The And", "Pizza"],
            threshold=0.5,
        )
        result = clf._keyword_match(req)

        assert result is None

    def test_keyword_match_no_model_call(self, clf, mock_pipeline):
        call_count_after_init = mock_pipeline.call_count

        req = ClassifyRequest(
            message="I love pizza",
            topic="Best Food",
            existing_labels=["Pizza", "Sushi", "Burger"],
            threshold=0.5,
        )
        result = clf.classify(req)

        assert mock_pipeline.call_count == call_count_after_init
        assert result.label == "Pizza"
        assert result.is_new is False


class TestLongestCommonSubstring:
    def test_empty_strings(self):
        assert VoteClassifier._longest_common_substring_len("", "hello") == 0
        assert VoteClassifier._longest_common_substring_len("hello", "") == 0
        assert VoteClassifier._longest_common_substring_len("", "") == 0

    def test_no_common_substring(self):
        assert VoteClassifier._longest_common_substring_len("abc", "def") == 0

    def test_full_match(self):
        assert VoteClassifier._longest_common_substring_len("hello", "hello") == 5

    def test_partial_match(self):
        assert VoteClassifier._longest_common_substring_len("sushi is great", "raw sushi") == 5

    def test_single_char_match(self):
        assert VoteClassifier._longest_common_substring_len("a", "a") == 1
        assert VoteClassifier._longest_common_substring_len("abc", "xbx") == 1
