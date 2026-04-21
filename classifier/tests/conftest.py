import sys
import os

from unittest.mock import MagicMock

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

mock_transformers = MagicMock()
sys.modules["transformers"] = mock_transformers

import pytest


@pytest.fixture
def mock_transformers_fixture():
    return mock_transformers


@pytest.fixture(autouse=True)
def reset_mock_transformers():
    yield
    mock_transformers.reset_mock()