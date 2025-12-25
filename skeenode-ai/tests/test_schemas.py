"""
Unit Tests for Schemas
"""

import pytest
from pydantic import ValidationError

from src.schemas import (
    Decision,
    HealthStatus,
    JobFeatures,
    PredictionRequest,
    PredictionResponse,
    BatchPredictionRequest,
)


class TestJobFeatures:
    """Test JobFeatures schema validation"""
    
    def test_valid_features(self):
        """Test valid job features"""
        features = JobFeatures(
            day_of_week=0,
            hour=12,
            job_type="SHELL",
            execution_count=10,
        )
        assert features.day_of_week == 0
        assert features.job_type == "SHELL"
    
    def test_job_type_normalized_to_uppercase(self):
        """Test job type is normalized to uppercase"""
        features = JobFeatures(
            day_of_week=0,
            hour=12,
            job_type="shell",
        )
        assert features.job_type == "SHELL"
    
    def test_invalid_day_of_week(self):
        """Test day_of_week validation (0-6)"""
        with pytest.raises(ValidationError):
            JobFeatures(day_of_week=7, hour=12, job_type="SHELL")
    
    def test_invalid_hour(self):
        """Test hour validation (0-23)"""
        with pytest.raises(ValidationError):
            JobFeatures(day_of_week=0, hour=25, job_type="SHELL")
    
    def test_invalid_job_type(self):
        """Test job type validation"""
        with pytest.raises(ValidationError):
            JobFeatures(day_of_week=0, hour=12, job_type="INVALID")
    
    def test_failure_rate_bounds(self):
        """Test failure_rate must be between 0 and 1"""
        with pytest.raises(ValidationError):
            JobFeatures(day_of_week=0, hour=12, job_type="SHELL", failure_rate=1.5)


class TestPredictionRequest:
    """Test PredictionRequest schema"""
    
    def test_valid_request(self):
        """Test valid prediction request"""
        request = PredictionRequest(
            job_id="job-123",
            features=JobFeatures(day_of_week=1, hour=14, job_type="DOCKER"),
        )
        assert request.job_id == "job-123"
        assert request.features.job_type == "DOCKER"
    
    def test_optional_request_id(self):
        """Test request_id is optional"""
        request = PredictionRequest(
            job_id="job-123",
            features=JobFeatures(day_of_week=1, hour=14, job_type="HTTP"),
        )
        assert request.request_id is None


class TestPredictionResponse:
    """Test PredictionResponse schema"""
    
    def test_valid_response(self):
        """Test valid prediction response"""
        response = PredictionResponse(
            job_id="job-123",
            failure_probability=0.3,
            confidence=0.85,
            decision=Decision.PROCEED,
            model_version="v1.0",
            latency_ms=5.2,
        )
        assert response.decision == "PROCEED"
    
    def test_probability_bounds(self):
        """Test probability must be between 0 and 1"""
        with pytest.raises(ValidationError):
            PredictionResponse(
                job_id="job-123",
                failure_probability=1.5,
                confidence=0.5,
                decision=Decision.PROCEED,
                model_version="v1.0",
                latency_ms=5.0,
            )


class TestBatchPredictionRequest:
    """Test batch prediction request limits"""
    
    def test_max_batch_size(self):
        """Test batch size limit of 100"""
        features = JobFeatures(day_of_week=0, hour=12, job_type="SHELL")
        requests = [
            PredictionRequest(job_id=f"job-{i}", features=features)
            for i in range(101)
        ]
        
        with pytest.raises(ValidationError):
            BatchPredictionRequest(predictions=requests)


class TestEnums:
    """Test enum values"""
    
    def test_health_status_values(self):
        """Test HealthStatus enum"""
        assert HealthStatus.HEALTHY == "healthy"
        assert HealthStatus.DEGRADED == "degraded"
        assert HealthStatus.UNHEALTHY == "unhealthy"
    
    def test_decision_values(self):
        """Test Decision enum"""
        assert Decision.PROCEED == "PROCEED"
        assert Decision.DELAY == "DELAY"
        assert Decision.ABORT == "ABORT"
