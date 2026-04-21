import logging

from unittest.mock import MagicMock, patch

import os

import pytest
from fastapi.testclient import TestClient

import main as main_mod
from schemas import ClassifyResponse


@pytest.fixture
def mock_classifier():
    clf = MagicMock()
    clf.model_name = "test-model"
    return clf


@pytest.fixture
def client(mock_classifier):
    with TestClient(main_mod.app) as c:
        main_mod.classifier = mock_classifier
        main_mod.semaphore = MagicMock()
        yield c, mock_classifier


class TestHealthEndpoint:
    def test_health_model_loaded(self, client):
        c, _ = client
        resp = c.get("/health")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "healthy"
        assert data["model_loaded"] is True
        assert data["model_name"] == "test-model"


class TestClassifyEndpoint:
    def test_classify_success(self, client):
        c, mock_clf = client
        mock_clf.classify.return_value = ClassifyResponse(
            label="Pizza", confidence=0.9, is_new=False,
            all_scores={"Pizza": 0.9, "Sushi": 0.1}
        )

        resp = c.post("/classify", json={
            "message": "I love pizza",
            "topic": "Best Food",
            "existing_labels": ["Pizza", "Sushi"],
            "threshold": 0.5,
        })

        assert resp.status_code == 200
        data = resp.json()
        assert data["label"] == "Pizza"
        assert data["confidence"] == 0.9
        assert data["is_new"] is False
        assert data["all_scores"]["Pizza"] == 0.9

    def test_classify_new_label(self, client):
        c, mock_clf = client
        mock_clf.classify.return_value = ClassifyResponse(
            label="Tacos", confidence=0.7, is_new=True
        )

        resp = c.post("/classify", json={
            "message": "Nothing beats tacos",
            "topic": "Best Food",
            "existing_labels": [],
            "threshold": 0.5,
        })

        assert resp.status_code == 200
        data = resp.json()
        assert data["is_new"] is True
        assert data["all_scores"] is None

    def test_classify_invalid_request(self, client):
        c, _ = client
        resp = c.post("/classify", json={
            "message": "",
            "topic": "test",
        })
        assert resp.status_code == 422

    def test_classify_missing_message(self, client):
        c, _ = client
        resp = c.post("/classify", json={
            "topic": "test",
        })
        assert resp.status_code == 422

    def test_classify_invalid_threshold(self, client):
        c, _ = client
        resp = c.post("/classify", json={
            "message": "hello",
            "topic": "test",
            "threshold": 2.0,
        })
        assert resp.status_code == 422

    def test_classify_registers_label(self, client):
        c, mock_clf = client
        mock_clf.classify.return_value = ClassifyResponse(
            label="Pizza", confidence=0.9, is_new=False
        )

        c.post("/classify", json={
            "message": "I love pizza",
            "topic": "Best Food",
            "topic_id": "test-uuid-123",
            "existing_labels": ["Pizza"],
            "threshold": 0.5,
        })

        mock_clf.register_label.assert_called_with("test-uuid-123", "Pizza")

    def test_classify_no_register_without_topic_id(self, client):
        c, mock_clf = client
        mock_clf.classify.return_value = ClassifyResponse(
            label="Pizza", confidence=0.9, is_new=False
        )

        c.post("/classify", json={
            "message": "I love pizza",
            "topic": "Best Food",
            "existing_labels": ["Pizza"],
            "threshold": 0.5,
        })

        mock_clf.register_label.assert_not_called()

    def test_classify_sanitized_error_response(self, client):
        c, mock_clf = client
        mock_clf.classify.side_effect = RuntimeError("secret internal error details")

        resp = c.post("/classify", json={
            "message": "hello",
            "topic": "test",
        })
        assert resp.status_code == 500
        data = resp.json()
        assert "secret internal error details" not in str(data)
        assert data["detail"] == "classification_error"

    def test_classify_invalid_topic_id_special_chars(self, client):
        c, _ = client
        resp = c.post("/classify", json={
            "message": "hello",
            "topic": "test",
            "topic_id": "bad!id@#",
        })
        assert resp.status_code == 422

    def test_classify_invalid_topic_id_too_long(self, client):
        c, _ = client
        resp = c.post("/classify", json={
            "message": "hello",
            "topic": "test",
            "topic_id": "x" * 256,
        })
        assert resp.status_code == 422


class TestLabelsEndpoint:
    def test_labels_empty(self, client):
        c, mock_clf = client
        mock_clf.get_labels.return_value = []

        resp = c.get("/labels?topic_id=test-topic")
        assert resp.status_code == 200
        data = resp.json()
        assert data["topic_id"] == "test-topic"
        assert data["labels"] == []

    def test_labels_with_data(self, client):
        c, mock_clf = client
        mock_clf.get_labels.return_value = ["Pizza", "Sushi"]

        resp = c.get("/labels?topic_id=test-topic")
        assert resp.status_code == 200
        data = resp.json()
        assert data["labels"] == ["Pizza", "Sushi"]


class TestModelNotLoaded:
    def test_classify_503_when_classifier_none(self):
        with TestClient(main_mod.app) as c:
            main_mod.classifier = None
            main_mod.semaphore = MagicMock()
            resp = c.post("/classify", json={
                "message": "hello", "topic": "test"
            })
            assert resp.status_code == 503
            assert resp.json()["detail"] == "model_not_loaded"

    def test_health_unhealthy_when_classifier_none(self):
        with TestClient(main_mod.app) as c:
            main_mod.classifier = None
            resp = c.get("/health")
            assert resp.status_code == 200
            data = resp.json()
            assert data["status"] == "unhealthy"
            assert data["model_loaded"] is False

    def test_labels_empty_when_classifier_none(self):
        with TestClient(main_mod.app) as c:
            main_mod.classifier = None
            resp = c.get("/labels?topic_id=test")
            assert resp.status_code == 200
            assert resp.json()["labels"] == []