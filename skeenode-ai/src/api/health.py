"""
API Routes for Health Checks and Model Management
"""

import logging
import time
from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException

from ..config import settings
from ..model_registry import ModelRegistry, get_registry
from ..schemas import (
    ActivateModelRequest,
    DependencyHealth,
    HealthResponse,
    HealthStatus,
    ModelInfo,
    ModelListResponse,
)

logger = logging.getLogger(__name__)

# Track service start time
_start_time = time.time()

router = APIRouter(tags=["Health & Management"])


@router.get(
    "/health",
    response_model=HealthResponse,
    summary="Health check",
    description="Check service health and dependencies",
)
async def health_check(
    registry: ModelRegistry = Depends(get_registry),
) -> HealthResponse:
    """Comprehensive health check with dependency status"""
    
    dependencies = []
    overall_status = HealthStatus.HEALTHY
    
    # Check Redis
    try:
        start = time.time()
        registry.redis.ping()
        latency = (time.time() - start) * 1000
        dependencies.append(DependencyHealth(
            name="redis",
            status=HealthStatus.HEALTHY,
            latency_ms=latency,
        ))
    except Exception as e:
        dependencies.append(DependencyHealth(
            name="redis",
            status=HealthStatus.UNHEALTHY,
            message=str(e),
        ))
        overall_status = HealthStatus.DEGRADED
    
    # Check model
    model_result = registry.get_model_for_prediction()
    model_loaded = model_result is not None
    active_version = model_result[0] if model_result else None
    
    if not model_loaded:
        overall_status = HealthStatus.DEGRADED
    
    return HealthResponse(
        status=overall_status,
        version=settings.service_name,
        uptime_seconds=time.time() - _start_time,
        dependencies=dependencies,
        model_loaded=model_loaded,
        active_model_version=active_version,
    )


@router.get("/ready")
async def readiness_check(
    registry: ModelRegistry = Depends(get_registry),
):
    """Kubernetes readiness probe"""
    model = registry.get_model_for_prediction()
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")
    return {"status": "ready"}


@router.get("/live")
async def liveness_check():
    """Kubernetes liveness probe"""
    return {"status": "alive"}


# ============================================
# Model Management Routes
# ============================================

model_router = APIRouter(prefix="/models", tags=["Model Management"])


@model_router.get(
    "",
    response_model=ModelListResponse,
    summary="List registered models",
)
async def list_models(
    registry: ModelRegistry = Depends(get_registry),
) -> ModelListResponse:
    """List all registered model versions"""
    versions = registry.list_versions()
    active_version = None
    
    models = []
    for v in versions:
        if v.is_active and v.traffic_weight > 0:
            active_version = v.version_id
        models.append(ModelInfo(
            version_id=v.version_id,
            model_type=v.model_type,
            created_at=datetime.fromtimestamp(v.created_at),
            is_active=v.is_active,
            traffic_weight=v.traffic_weight,
            metrics=v.metrics,
            features=v.features,
        ))
    
    return ModelListResponse(models=models, active_version=active_version)


@model_router.post(
    "/activate",
    summary="Activate a model version",
)
async def activate_model(
    request: ActivateModelRequest,
    registry: ModelRegistry = Depends(get_registry),
):
    """Activate a specific model version for traffic"""
    try:
        registry.activate_version(request.version_id, request.traffic_weight)
        return {"status": "activated", "version_id": request.version_id}
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@model_router.post(
    "/rollback/{version_id}",
    summary="Rollback to a model version",
)
async def rollback_model(
    version_id: str,
    registry: ModelRegistry = Depends(get_registry),
):
    """Rollback to a previous model version"""
    try:
        registry.rollback(version_id)
        return {"status": "rolled_back", "version_id": version_id}
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@model_router.delete(
    "/{version_id}",
    summary="Delete a model version",
)
async def delete_model(
    version_id: str,
    registry: ModelRegistry = Depends(get_registry),
):
    """Delete a non-active model version"""
    try:
        registry.delete_version(version_id)
        return {"status": "deleted", "version_id": version_id}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
