"""
Training API Routes

Endpoints for triggering model training and checking status.
Uses Redis for training status to support distributed deployments.
"""

import json
import logging
from typing import Optional

import redis
from fastapi import APIRouter, BackgroundTasks, Depends, HTTPException
from pydantic import BaseModel, Field

from ..config import settings
from ..data.training import TrainingConfig, TrainingPipeline, TrainingResult

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/training", tags=["Training"])


# Redis-backed training status
class TrainingStatusStore:
    """Redis-backed training status for distributed deployments"""
    
    STATUS_KEY = "skeenode:training:status"
    RESULT_KEY = "skeenode:training:result"
    LOCK_KEY = "skeenode:training:lock"
    LOCK_TTL = 3600  # 1 hour max training time
    
    def __init__(self, redis_url: str = None):
        url = redis_url or settings.redis_url
        self.redis = redis.from_url(url)
    
    def is_running(self) -> bool:
        """Check if training is currently running"""
        return self.redis.exists(self.LOCK_KEY)
    
    def acquire_lock(self) -> bool:
        """Try to acquire training lock"""
        return self.redis.set(self.LOCK_KEY, "1", nx=True, ex=self.LOCK_TTL)
    
    def release_lock(self):
        """Release training lock"""
        self.redis.delete(self.LOCK_KEY)
    
    def set_result(self, result: dict):
        """Store training result"""
        self.redis.set(self.RESULT_KEY, json.dumps(result))
    
    def get_result(self) -> Optional[dict]:
        """Get last training result"""
        data = self.redis.get(self.RESULT_KEY)
        if data:
            return json.loads(data)
        return None


# Global status store
_status_store: Optional[TrainingStatusStore] = None


def get_status_store() -> TrainingStatusStore:
    """Get or create status store"""
    global _status_store
    if _status_store is None:
        _status_store = TrainingStatusStore()
    return _status_store


class TrainingRequest(BaseModel):
    """Request to trigger training"""
    database_url: str = Field(description="PostgreSQL connection string")
    lookback_days: int = Field(default=90, ge=7, le=365)
    activate_on_success: bool = Field(default=False)
    min_accuracy: float = Field(default=0.7, ge=0.5, le=1.0)


class TrainingStatusResponse(BaseModel):
    """Training status response"""
    running: bool
    last_result: Optional[dict] = None


@router.post(
    "/start",
    summary="Start model training",
    description="Triggers async model training from historical data",
)
async def start_training(
    request: TrainingRequest,
    background_tasks: BackgroundTasks,
    status_store: TrainingStatusStore = Depends(get_status_store),
):
    """
    Start training in background. Training collects data from PostgreSQL,
    engineers features, trains XGBoost model, and registers if successful.
    """
    if status_store.is_running():
        raise HTTPException(
            status_code=409,
            detail="Training already in progress",
        )
    
    if not status_store.acquire_lock():
        raise HTTPException(
            status_code=409,
            detail="Failed to acquire training lock",
        )
    
    # Run training in background
    background_tasks.add_task(
        _run_training_task,
        request.database_url,
        request.lookback_days,
        request.activate_on_success,
        request.min_accuracy,
        status_store,
    )
    
    return {"status": "started", "message": "Training started in background"}


@router.get(
    "/status",
    response_model=TrainingStatusResponse,
    summary="Get training status",
)
async def get_training_status(
    status_store: TrainingStatusStore = Depends(get_status_store),
):
    """Get current training status and last result"""
    return TrainingStatusResponse(
        running=status_store.is_running(),
        last_result=status_store.get_result(),
    )


async def _run_training_task(
    database_url: str,
    lookback_days: int,
    activate: bool,
    min_accuracy: float,
    status_store: TrainingStatusStore,
):
    """Background training task"""
    try:
        config = TrainingConfig(
            database_url=database_url,
            lookback_days=lookback_days,
            activate_on_success=activate,
            min_accuracy=min_accuracy,
        )
        pipeline = TrainingPipeline(config)
        result = pipeline.train()
        
        status_store.set_result({
            "success": result.success,
            "version_id": result.version_id,
            "metrics": result.metrics,
            "samples_used": result.samples_used,
            "training_time_seconds": result.training_time_seconds,
            "error": result.error,
        })
    except Exception as e:
        logger.error(f"Training task failed: {e}")
        status_store.set_result({
            "success": False,
            "error": str(e),
        })
    finally:
        status_store.release_lock()
