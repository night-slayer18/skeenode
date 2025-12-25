"""
Skeenode AI Service - Production Entrypoint

FastAPI application with proper middleware stack,
route organization, and error handling.
"""

import logging
import os
from contextlib import asynccontextmanager
from typing import AsyncGenerator

import numpy as np
import pandas as pd
import uvicorn
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from xgboost import XGBClassifier

from .api.health import model_router, router as health_router
from .api.predictions import router as prediction_router
from .api.training import router as training_router
from .config import settings
from .middleware.logging import RequestLoggingMiddleware, setup_logging
from .middleware.rate_limit import RateLimitMiddleware
from .model_registry import init_registry

# Setup logging first
setup_logging()
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator:
    """Application lifecycle management"""
    logger.info(f"Starting {settings.service_name} in {settings.environment} mode")
    
    # Initialize model registry
    try:
        registry = init_registry(
            redis_url=settings.redis_url,
            models_dir=settings.model_path,
        )
        
        # Check if we have any models, if not train a bootstrap model
        if not registry.list_versions():
            logger.info("No models found, training bootstrap model...")
            model = train_bootstrap_model()
            registry.register_model(
                model=model,
                metrics={"bootstrap": True, "accuracy": 0.0},
                features=[
                    "day_of_week",
                    "hour",
                    "job_type_len",
                    "execution_count",
                    "avg_duration_ms",
                    "failure_rate",
                ],
                model_type="xgboost",
                activate=True,
            )
            logger.info("Bootstrap model registered and activated")
        else:
            logger.info(f"Found {len(registry.list_versions())} registered model(s)")
    
    except Exception as e:
        logger.warning(f"Failed to initialize model registry: {e}")
        logger.warning("Service starting in degraded mode")
    
    yield
    
    logger.info("Shutting down...")


def train_bootstrap_model() -> XGBClassifier:
    """Train a bootstrap model for initial predictions"""
    n_samples = 1000
    X = pd.DataFrame({
        "day_of_week": np.random.randint(0, 7, n_samples),
        "hour": np.random.randint(0, 24, n_samples),
        "job_type_len": np.random.randint(4, 10, n_samples),
        # Additional operational features expected by PredictionService
        "execution_count": np.random.randint(0, 200, n_samples),
        "avg_duration_ms": np.random.exponential(scale=1000.0, size=n_samples),
        "failure_rate": np.random.random(n_samples),
    })
    
    # Simulate failure patterns
    y = []
    for _, row in X.iterrows():
        prob = 0.1
        if row["day_of_week"] >= 5:  # Weekend
            prob += 0.3
        if row["hour"] < 6:  # Late night
            prob += 0.2
        y.append(1 if np.random.random() < prob else 0)
    
    model = XGBClassifier(n_estimators=100, max_depth=3, eval_metric="logloss")
    model.fit(X, y)
    return model


def create_app() -> FastAPI:
    """Application factory"""
    app = FastAPI(
        title="Skeenode Intelligence Service",
        description="AI-powered job failure prediction and analysis",
        version="1.0.0",
        lifespan=lifespan,
        docs_url="/docs" if settings.debug else None,
        redoc_url="/redoc" if settings.debug else None,
    )
    
    # Add CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"] if settings.debug else [],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    
    # Add custom middleware
    app.add_middleware(RequestLoggingMiddleware)
    app.add_middleware(RateLimitMiddleware)
    
    # Register routes
    app.include_router(health_router)
    app.include_router(model_router)
    app.include_router(prediction_router)
    app.include_router(training_router)
    
    # Global exception handler
    @app.exception_handler(Exception)
    async def global_exception_handler(request: Request, exc: Exception):
        logger.error(f"Unhandled exception: {exc}", exc_info=True)
        return JSONResponse(
            status_code=500,
            content={
                "error": "Internal server error",
                "detail": str(exc) if settings.debug else "An unexpected error occurred",
            },
        )
    
    return app


# Create application instance
app = create_app()


if __name__ == "__main__":
    uvicorn.run(
        "src.app:app",
        host=settings.host,
        port=settings.port,
        workers=settings.workers if settings.environment != "development" else 1,
        reload=settings.debug,
    )
