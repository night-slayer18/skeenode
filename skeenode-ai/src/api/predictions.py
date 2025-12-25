"""
API Routes for Predictions

Organized FastAPI routes with dependency injection,
proper error handling, and documentation.
"""

import logging
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import JSONResponse

from ..schemas import (
    BatchPredictionRequest,
    BatchPredictionResponse,
    ErrorResponse,
    PredictionRequest,
    PredictionResponse,
)
from ..services.prediction import PredictionService, get_prediction_service

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/predict", tags=["Predictions"])


@router.post(
    "/failure",
    response_model=PredictionResponse,
    responses={
        503: {"model": ErrorResponse, "description": "Model not available"},
        500: {"model": ErrorResponse, "description": "Prediction failed"},
    },
    summary="Predict job failure probability",
    description="Analyzes job features and predicts the probability of failure",
)
async def predict_failure(
    request: PredictionRequest,
    service: PredictionService = Depends(get_prediction_service),
) -> PredictionResponse:
    """
    Predict the probability of job failure.
    
    Returns a decision:
    - PROCEED: Low failure probability (< 40%)
    - DELAY: Medium failure probability (40-70%)
    - ABORT: High failure probability (> 70%)
    """
    try:
        return service.predict(request)
    except RuntimeError as e:
        if "No active model" in str(e):
            raise HTTPException(status_code=503, detail=str(e))
        raise HTTPException(status_code=500, detail=str(e))


@router.post(
    "/batch",
    response_model=BatchPredictionResponse,
    responses={
        503: {"model": ErrorResponse, "description": "Model not available"},
        422: {"model": ErrorResponse, "description": "Validation error"},
    },
    summary="Batch predict failures",
    description="Process multiple predictions in a single request (max 100)",
)
async def predict_batch(
    request: BatchPredictionRequest,
    service: PredictionService = Depends(get_prediction_service),
) -> BatchPredictionResponse:
    """Batch prediction endpoint for high-throughput scenarios"""
    try:
        results, total_latency = service.predict_batch(request.predictions)
        return BatchPredictionResponse(
            results=results,
            total_latency_ms=total_latency,
        )
    except RuntimeError as e:
        if "No active model" in str(e):
            raise HTTPException(status_code=503, detail=str(e))
        raise HTTPException(status_code=500, detail=str(e))
