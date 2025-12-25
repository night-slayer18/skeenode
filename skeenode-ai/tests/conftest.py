"""
Pytest Configuration and Fixtures
"""

import pytest
from fastapi.testclient import TestClient

# Mock settings before importing app
import os
os.environ["REDIS_URL"] = "redis://localhost:6379"
os.environ["ENVIRONMENT"] = "testing"
os.environ["RATE_LIMIT_ENABLED"] = "false"

from src.app import app


@pytest.fixture
def client():
    """FastAPI test client"""
    with TestClient(app) as c:
        yield c


@pytest.fixture
def mock_features():
    """Sample job features for testing"""
    return {
        "day_of_week": 1,
        "hour": 14,
        "job_type": "SHELL",
        "execution_count": 5,
        "avg_duration_ms": 1000.0,
        "failure_rate": 0.1,
    }


@pytest.fixture
def prediction_request(mock_features):
    """Sample prediction request"""
    return {
        "job_id": "test-job-123",
        "features": mock_features,
        "request_id": "req-001",
    }
