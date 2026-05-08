import sys
import os

from unittest.mock import MagicMock

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

mock_transformers = MagicMock()
sys.modules["transformers"] = mock_transformers

mock_keybert = MagicMock()
sys.modules["keybert"] = mock_keybert

mock_sentence_transformers = MagicMock()
sys.modules["sentence_transformers"] = mock_sentence_transformers

import pytest


@pytest.fixture
def mock_transformers_fixture():
    return mock_transformers


@pytest.fixture
def mock_keybert_fixture():
    return mock_keybert


@pytest.fixture(autouse=True)
def reset_mocks():
    yield
    mock_transformers.reset_mock()
    mock_keybert.reset_mock()
    mock_sentence_transformers.reset_mock()
