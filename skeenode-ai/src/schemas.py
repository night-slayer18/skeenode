"""
Pydantic Schemas for API Request/Response Models

Strict type validation with comprehensive field documentation.
"""

from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field, validator


class HealthStatus(str, Enum):
    """Service health states"""
    HEALTHY = "healthy"
    DEGRADED = "degraded"
    UNHEALTHY = "unhealthy"


class Decision(str, Enum):
    """Prediction decision outcomes"""
    PROCEED = "PROCEED"
    DELAY = "DELAY"
    ABORT = "ABORT"


# ============================================
# Health Check Schemas
# ============================================

class DependencyHealth(BaseModel):
    """Health status of a single dependency"""
    name: str
    status: HealthStatus
    latency_ms: Optional[float] = None
    message: Optional[str] = None


class HealthResponse(BaseModel):
    """Health check response"""
    status: HealthStatus
    service: str = "skeenode-ai"
    version: str
    uptime_seconds: float
    dependencies: List[DependencyHealth] = []
    model_loaded: bool
    active_model_version: Optional[str] = None


# ============================================
# Prediction Schemas
# ============================================

class JobFeatures(BaseModel):
    """Features extracted from a job for prediction"""
    day_of_week: int = Field(ge=0, le=6, description="Day of week (0=Monday)")
    hour: int = Field(ge=0, le=23, description="Hour of day (0-23)")
    job_type: str = Field(description="Job type (SHELL, DOCKER, HTTP)")
    execution_count: int = Field(default=0, ge=0, description="Previous execution count")
    avg_duration_ms: Optional[float] = Field(default=None, description="Average duration")
    failure_rate: Optional[float] = Field(default=None, ge=0, le=1, description="Historical failure rate")
    
    @validator("job_type")
    def validate_job_type(cls, v):
        allowed = {"SHELL", "DOCKER", "HTTP", "KUBERNETES"}
        if v.upper() not in allowed:
            raise ValueError(f"job_type must be one of {allowed}")
        return v.upper()


class PredictionRequest(BaseModel):
    """Request for failure prediction"""
    job_id: str = Field(description="Unique job identifier")
    features: JobFeatures = Field(description="Job features for prediction")
    request_id: Optional[str] = Field(default=None, description="Optional trace ID")


class PredictionResponse(BaseModel):
    """Prediction result"""
    job_id: str
    request_id: Optional[str] = None
    failure_probability: float = Field(ge=0, le=1)
    confidence: float = Field(ge=0, le=1)
    decision: Decision
    model_version: str
    latency_ms: float
    cached: bool = False
    
    class Config:
        use_enum_values = True


class BatchPredictionRequest(BaseModel):
    """Batch prediction request"""
    predictions: List[PredictionRequest] = Field(max_items=100)


class BatchPredictionResponse(BaseModel):
    """Batch prediction response"""
    results: List[PredictionResponse]
    total_latency_ms: float


# ============================================
# Model Management Schemas
# ============================================

class ModelInfo(BaseModel):
    """Information about a registered model"""
    version_id: str
    model_type: str
    created_at: datetime
    is_active: bool
    traffic_weight: float = Field(ge=0, le=1)
    metrics: Dict[str, float] = {}
    features: List[str] = []


class ModelListResponse(BaseModel):
    """List of registered models"""
    models: List[ModelInfo]
    active_version: Optional[str] = None


class ActivateModelRequest(BaseModel):
    """Request to activate a model version"""
    version_id: str
    traffic_weight: float = Field(default=1.0, ge=0, le=1)


# ============================================
# Error Schemas
# ============================================

class ErrorDetail(BaseModel):
    """Error response detail"""
    code: str
    message: str
    field: Optional[str] = None


class ErrorResponse(BaseModel):
    """Standard error response"""
    error: str
    details: List[ErrorDetail] = []
    request_id: Optional[str] = None
    timestamp: datetime = Field(default_factory=datetime.utcnow)
