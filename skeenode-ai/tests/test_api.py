"""
API Endpoint Tests
"""

import pytest
from fastapi import status


class TestHealthEndpoints:
    """Test health check endpoints"""
    
    def test_liveness_probe(self, client):
        """Test /live returns 200"""
        response = client.get("/live")
        assert response.status_code == status.HTTP_200_OK
        assert response.json()["status"] == "alive"
    
    def test_readiness_probe_degraded_without_model(self, client):
        """Test /ready returns appropriate status"""
        response = client.get("/ready")
        # May be 503 if no model loaded
        assert response.status_code in [status.HTTP_200_OK, status.HTTP_503_SERVICE_UNAVAILABLE]
    
    def test_health_check_returns_details(self, client):
        """Test /health returns comprehensive info"""
        response = client.get("/health")
        data = response.json()
        
        assert "status" in data
        assert "service" in data
        assert "uptime_seconds" in data
        assert "model_loaded" in data
        assert data["service"] == "skeenode-ai"


class TestPredictionEndpoints:
    """Test prediction API endpoints"""
    
    def test_predict_requires_valid_features(self, client):
        """Test validation on prediction request"""
        response = client.post("/predict/failure", json={
            "job_id": "test",
            "features": {
                "day_of_week": 10,  # Invalid: must be 0-6
                "hour": 12,
                "job_type": "SHELL",
            }
        })
        assert response.status_code == status.HTTP_422_UNPROCESSABLE_ENTITY
    
    def test_predict_rejects_invalid_job_type(self, client):
        """Test job type validation"""
        response = client.post("/predict/failure", json={
            "job_id": "test",
            "features": {
                "day_of_week": 1,
                "hour": 12,
                "job_type": "INVALID_TYPE",
            }
        })
        assert response.status_code == status.HTTP_422_UNPROCESSABLE_ENTITY
    
    def test_predict_with_valid_request(self, client, prediction_request):
        """Test successful prediction (may fail if model not loaded)"""
        response = client.post("/predict/failure", json=prediction_request)
        
        # Either succeeds with prediction or fails with 503 (no model)
        if response.status_code == status.HTTP_200_OK:
            data = response.json()
            assert "job_id" in data
            assert "failure_probability" in data
            assert "decision" in data
            assert 0 <= data["failure_probability"] <= 1
            assert data["decision"] in ["PROCEED", "DELAY", "ABORT"]
        else:
            assert response.status_code == status.HTTP_503_SERVICE_UNAVAILABLE
    
    def test_batch_prediction_limit(self, client, prediction_request):
        """Test batch endpoint respects max limit"""
        # Create 101 requests (exceeds limit of 100)
        requests = [prediction_request for _ in range(101)]
        response = client.post("/predict/batch", json={"predictions": requests})
        assert response.status_code == status.HTTP_422_UNPROCESSABLE_ENTITY


class TestModelManagement:
    """Test model management endpoints"""
    
    def test_list_models(self, client):
        """Test listing model versions"""
        response = client.get("/models")
        assert response.status_code == status.HTTP_200_OK
        data = response.json()
        assert "models" in data
        assert isinstance(data["models"], list)
    
    def test_activate_nonexistent_model(self, client):
        """Test activating non-existent model returns 404"""
        response = client.post("/models/activate", json={
            "version_id": "nonexistent-version-123",
            "traffic_weight": 1.0,
        })
        assert response.status_code == status.HTTP_404_NOT_FOUND
    
    def test_rollback_nonexistent_model(self, client):
        """Test rollback to non-existent version returns 404"""
        response = client.post("/models/rollback/nonexistent-version")
        assert response.status_code == status.HTTP_404_NOT_FOUND
