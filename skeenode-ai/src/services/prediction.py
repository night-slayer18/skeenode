"""
Prediction Service

Core business logic for failure predictions with caching,
metrics collection, and model management.
"""

import hashlib
import json
import logging
import time
from typing import Optional, Tuple

import numpy as np
import pandas as pd
import redis

from ..config import settings
from ..schemas import Decision, JobFeatures, PredictionRequest, PredictionResponse
from ..model_registry import ModelRegistry, get_registry

logger = logging.getLogger(__name__)


class PredictionService:
    """
    Production prediction service with:
    - Model version management
    - Prediction caching
    - Latency tracking
    - Decision thresholds
    """
    
    # Decision thresholds
    ABORT_THRESHOLD = 0.7
    DELAY_THRESHOLD = 0.4
    
    def __init__(
        self,
        registry: Optional[ModelRegistry] = None,
        redis_client: Optional[redis.Redis] = None,
    ):
        self.registry = registry or get_registry()
        self.redis = redis_client
        if self.redis is None and settings.prediction_caching_enabled:
            try:
                self.redis = redis.from_url(settings.redis_url)
            except Exception as e:
                logger.warning(f"Failed to connect to Redis for caching: {e}")
                self.redis = None
    
    def predict(self, request: PredictionRequest) -> PredictionResponse:
        """Execute a prediction with caching and metrics"""
        start_time = time.time()
        
        # Check cache first
        cache_key = self._cache_key(request)
        if settings.prediction_caching_enabled and self.redis:
            cached = self._get_cached(cache_key)
            if cached:
                cached.latency_ms = (time.time() - start_time) * 1000
                cached.cached = True
                return cached
        
        # Get model for prediction
        result = self.registry.get_model_for_prediction()
        if result is None:
            raise RuntimeError("No active model available")
        
        version_id, model = result
        
        # Prepare features
        features_df = self._prepare_features(request.features)
        
        # Predict
        try:
            prob_fail = model.predict_proba(features_df)[0][1]
        except Exception as e:
            logger.error(f"Prediction failed: {e}")
            raise RuntimeError(f"Model prediction failed: {e}")
        
        # Determine decision
        decision = self._make_decision(prob_fail)
        
        # Calculate confidence (simple heuristic based on probability extremity)
        confidence = abs(prob_fail - 0.5) * 2  # 0.5 -> 0, 0/1 -> 1
        
        latency_ms = (time.time() - start_time) * 1000
        
        response = PredictionResponse(
            job_id=request.job_id,
            request_id=request.request_id,
            failure_probability=float(prob_fail),
            confidence=float(confidence),
            decision=decision,
            model_version=version_id,
            latency_ms=latency_ms,
            cached=False,
        )
        
        # Cache response
        if settings.prediction_caching_enabled and self.redis:
            self._cache_response(cache_key, response)
        
        logger.info(
            f"Prediction: job={request.job_id} prob={prob_fail:.4f} "
            f"decision={decision} version={version_id} latency={latency_ms:.2f}ms"
        )
        
        return response
    
    def predict_batch(
        self, requests: list[PredictionRequest]
    ) -> Tuple[list[PredictionResponse], float]:
        """Execute batch predictions"""
        start_time = time.time()
        results = [self.predict(req) for req in requests]
        total_latency = (time.time() - start_time) * 1000
        return results, total_latency
    
    def _prepare_features(self, features: JobFeatures) -> pd.DataFrame:
        """Convert JobFeatures to model input format"""
        return pd.DataFrame([{
            "day_of_week": features.day_of_week,
            "hour": features.hour,
            "job_type_len": len(features.job_type),
            "execution_count": features.execution_count,
            "avg_duration_ms": features.avg_duration_ms or 0,
            "failure_rate": features.failure_rate or 0,
        }])
    
    def _make_decision(self, probability: float) -> Decision:
        """Determine decision based on failure probability"""
        if probability >= self.ABORT_THRESHOLD:
            return Decision.ABORT
        elif probability >= self.DELAY_THRESHOLD:
            return Decision.DELAY
        return Decision.PROCEED
    
    def _cache_key(self, request: PredictionRequest) -> str:
        """Generate cache key for request"""
        key_data = {
            "job_id": request.job_id,
            "features": request.features.dict(),
        }
        key_hash = hashlib.sha256(json.dumps(key_data, sort_keys=True).encode()).hexdigest()
        return f"prediction:{key_hash[:16]}"
    
    def _get_cached(self, cache_key: str) -> Optional[PredictionResponse]:
        """Get cached prediction"""
        try:
            data = self.redis.get(cache_key)
            if data:
                return PredictionResponse.parse_raw(data)
        except Exception as e:
            logger.warning(f"Cache read failed: {e}")
        return None
    
    def _cache_response(self, cache_key: str, response: PredictionResponse) -> None:
        """Cache prediction response"""
        try:
            self.redis.setex(
                cache_key,
                settings.cache_ttl_seconds,
                response.json(),
            )
        except Exception as e:
            logger.warning(f"Cache write failed: {e}")


# Global service instance
_service: Optional[PredictionService] = None


def get_prediction_service() -> PredictionService:
    """Get or create the global prediction service"""
    global _service
    if _service is None:
        _service = PredictionService()
    return _service
