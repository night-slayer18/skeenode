"""
Model Registry for AI Predictions

Production-grade model versioning with A/B testing support,
hot-reload capability, and Redis-backed version storage.
"""

import os
import json
import time
import hashlib
import pickle
from pathlib import Path
from typing import Dict, Optional, Any, List
from dataclasses import dataclass, asdict
from datetime import datetime
import threading
import logging

import redis
import numpy as np
from sklearn.base import BaseEstimator

logger = logging.getLogger(__name__)


@dataclass
class ModelVersion:
    """Metadata for a model version"""
    version_id: str
    model_path: str
    created_at: float
    metrics: Dict[str, float]
    is_active: bool
    traffic_weight: float  # For A/B testing (0.0 to 1.0)
    features: List[str]
    model_type: str
    
    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ModelVersion":
        return cls(**data)


class ModelRegistry:
    """
    Production model registry with:
    - Version management
    - A/B testing support
    - Hot reloading
    - Redis-backed storage
    """
    
    REGISTRY_KEY = "skeenode:model_registry"
    ACTIVE_MODEL_KEY = "skeenode:active_model"
    
    def __init__(
        self,
        redis_url: str = None,
        models_dir: str = "./models",
        auto_reload_interval: int = 30,
    ):
        redis_url = redis_url or os.getenv("REDIS_URL", "redis://localhost:6379")
        self.redis = redis.from_url(redis_url)
        self.models_dir = Path(models_dir)
        self.models_dir.mkdir(parents=True, exist_ok=True)
        
        self._models: Dict[str, BaseEstimator] = {}
        self._versions: Dict[str, ModelVersion] = {}
        self._lock = threading.RLock()
        
        # Start auto-reload thread
        if auto_reload_interval > 0:
            self._reload_thread = threading.Thread(
                target=self._auto_reload_loop,
                args=(auto_reload_interval,),
                daemon=True,
            )
            self._reload_thread.start()
        
        # Load existing versions
        self._load_versions_from_redis()
    
    def register_model(
        self,
        model: BaseEstimator,
        metrics: Dict[str, float],
        features: List[str],
        model_type: str = "xgboost",
        activate: bool = False,
        traffic_weight: float = 0.0,
    ) -> str:
        """Register a new model version"""
        
        # Generate version ID based on model hash
        model_bytes = pickle.dumps(model)
        version_id = hashlib.sha256(model_bytes).hexdigest()[:12]
        version_id = f"v_{version_id}_{int(time.time())}"
        
        # Save model to disk
        model_path = self.models_dir / f"{version_id}.pkl"
        with open(model_path, "wb") as f:
            pickle.dump(model, f)
        
        # Create version metadata
        version = ModelVersion(
            version_id=version_id,
            model_path=str(model_path),
            created_at=time.time(),
            metrics=metrics,
            is_active=activate,
            traffic_weight=traffic_weight,
            features=features,
            model_type=model_type,
        )
        
        # Store in Redis
        self._store_version(version)
        
        # Load into memory
        with self._lock:
            self._models[version_id] = model
            self._versions[version_id] = version
        
        if activate:
            self.activate_version(version_id)
        
        logger.info(f"Registered model version: {version_id}")
        return version_id
    
    def activate_version(self, version_id: str, traffic_weight: float = 1.0) -> None:
        """Activate a model version for serving"""
        with self._lock:
            if version_id not in self._versions:
                raise ValueError(f"Unknown version: {version_id}")
            
            # Deactivate other versions if setting traffic to 1.0
            if traffic_weight >= 1.0:
                for v in self._versions.values():
                    v.is_active = False
                    v.traffic_weight = 0.0
            
            self._versions[version_id].is_active = True
            self._versions[version_id].traffic_weight = traffic_weight
            
            # Update Redis
            self._store_version(self._versions[version_id])
            self.redis.set(self.ACTIVE_MODEL_KEY, version_id)
        
        logger.info(f"Activated model version: {version_id} with weight {traffic_weight}")
    
    def get_model_for_prediction(self) -> Optional[tuple[str, BaseEstimator]]:
        """
        Get a model for prediction, supporting A/B testing.
        Returns (version_id, model) tuple.
        """
        with self._lock:
            active_versions = [
                v for v in self._versions.values()
                if v.is_active and v.traffic_weight > 0
            ]
            
            if not active_versions:
                return None
            
            # Simple weighted random selection for A/B testing
            weights = [v.traffic_weight for v in active_versions]
            total = sum(weights)
            weights = [w / total for w in weights]
            
            rand = np.random.random()
            cumsum = 0
            selected = active_versions[0]
            for v, w in zip(active_versions, weights):
                cumsum += w
                if rand < cumsum:
                    selected = v
                    break
            
            model = self._models.get(selected.version_id)
            if model is None:
                model = self._load_model(selected)
                self._models[selected.version_id] = model
            
            return selected.version_id, model
    
    def rollback(self, version_id: str) -> None:
        """Rollback to a previous version"""
        self.activate_version(version_id, traffic_weight=1.0)
        logger.info(f"Rolled back to version: {version_id}")
    
    def list_versions(self) -> List[ModelVersion]:
        """List all registered versions"""
        with self._lock:
            return list(self._versions.values())
    
    def get_version(self, version_id: str) -> Optional[ModelVersion]:
        """Get a specific version's metadata"""
        with self._lock:
            return self._versions.get(version_id)
    
    def delete_version(self, version_id: str) -> None:
        """Delete a model version"""
        with self._lock:
            if version_id not in self._versions:
                return
            
            version = self._versions[version_id]
            
            # Cannot delete active version
            if version.is_active:
                raise ValueError("Cannot delete active version")
            
            # Remove from disk
            model_path = Path(version.model_path)
            if model_path.exists():
                model_path.unlink()
            
            # Remove from Redis
            self.redis.hdel(self.REGISTRY_KEY, version_id)
            
            # Remove from memory
            del self._versions[version_id]
            if version_id in self._models:
                del self._models[version_id]
        
        logger.info(f"Deleted model version: {version_id}")
    
    def _store_version(self, version: ModelVersion) -> None:
        """Store version metadata in Redis"""
        self.redis.hset(
            self.REGISTRY_KEY,
            version.version_id,
            json.dumps(version.to_dict()),
        )
    
    def _load_versions_from_redis(self) -> None:
        """Load all version metadata from Redis"""
        versions = self.redis.hgetall(self.REGISTRY_KEY)
        for version_id, data in versions.items():
            version_id = version_id.decode() if isinstance(version_id, bytes) else version_id
            data = data.decode() if isinstance(data, bytes) else data
            version = ModelVersion.from_dict(json.loads(data))
            self._versions[version_id] = version
    
    def _load_model(self, version: ModelVersion) -> BaseEstimator:
        """Load a model from disk"""
        with open(version.model_path, "rb") as f:
            return pickle.load(f)
    
    def _auto_reload_loop(self, interval: int) -> None:
        """Background thread to reload versions from Redis"""
        while True:
            time.sleep(interval)
            try:
                self._load_versions_from_redis()
            except Exception as e:
                logger.error(f"Failed to reload versions: {e}")


# Global registry instance
_registry: Optional[ModelRegistry] = None


def get_registry() -> ModelRegistry:
    """Get or create the global model registry"""
    global _registry
    if _registry is None:
        _registry = ModelRegistry()
    return _registry


def init_registry(redis_url: str = None, models_dir: str = "./models") -> ModelRegistry:
    """Initialize the global model registry"""
    global _registry
    _registry = ModelRegistry(redis_url=redis_url, models_dir=models_dir)
    return _registry
