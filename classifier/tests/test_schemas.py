import pytest
from pydantic import ValidationError

from schemas import ClassifyRequest, ClassifyResponse


class TestClassifyRequest:
    def test_valid_request(self):
        req = ClassifyRequest(message="hello", topic="test")
        assert req.message == "hello"
        assert req.topic == "test"
        assert req.existing_labels == []
        assert req.threshold == 0.5

    def test_custom_threshold(self):
        req = ClassifyRequest(message="hello", topic="test", threshold=0.8)
        assert req.threshold == 0.8

    def test_empty_message_rejected(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="", topic="test")

    def test_message_too_long(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="x" * 513, topic="test")

    def test_message_max_length(self):
        req = ClassifyRequest(message="x" * 512, topic="test")
        assert len(req.message) == 512

    def test_empty_topic_rejected(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="")

    def test_topic_too_long(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="x" * 256)

    def test_threshold_below_zero(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="test", threshold=-0.1)

    def test_threshold_above_one(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="test", threshold=1.1)

    def test_threshold_boundary_zero(self):
        req = ClassifyRequest(message="hello", topic="test", threshold=0.0)
        assert req.threshold == 0.0

    def test_threshold_boundary_one(self):
        req = ClassifyRequest(message="hello", topic="test", threshold=1.0)
        assert req.threshold == 1.0

    def test_existing_labels_provided(self):
        req = ClassifyRequest(
            message="hello", topic="test", existing_labels=["Pizza", "Sushi"]
        )
        assert req.existing_labels == ["Pizza", "Sushi"]

    def test_existing_labels_default_factory(self):
        req1 = ClassifyRequest(message="hello", topic="test")
        req2 = ClassifyRequest(message="hello", topic="test")
        req1.existing_labels.append("Pizza")
        assert req2.existing_labels == []

    def test_topic_id_valid(self):
        req = ClassifyRequest(message="hello", topic="test", topic_id="abc-123_xyz")
        assert req.topic_id == "abc-123_xyz"

    def test_topic_id_none_by_default(self):
        req = ClassifyRequest(message="hello", topic="test")
        assert req.topic_id is None

    def test_topic_id_special_chars_rejected(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="test", topic_id="bad!id@#")

    def test_topic_id_too_long_rejected(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="test", topic_id="x" * 256)

    def test_topic_id_spaces_rejected(self):
        with pytest.raises(ValidationError):
            ClassifyRequest(message="hello", topic="test", topic_id="has spaces")


class TestClassifyResponse:
    def test_existing_label_response(self):
        resp = ClassifyResponse(
            label="Pizza", confidence=0.9, is_new=False, all_scores={"Pizza": 0.9, "Sushi": 0.1}
        )
        assert resp.is_new is False
        assert resp.all_scores is not None

    def test_new_label_response(self):
        resp = ClassifyResponse(label="Tacos", confidence=0.7, is_new=True)
        assert resp.is_new is True
        assert resp.all_scores is None

    def test_fallback_response(self):
        resp = ClassifyResponse(
            label="Short Message Title", confidence=0.0, is_new=True
        )
        assert resp.confidence == 0.0
        assert resp.is_new is True
        assert resp.all_scores is None