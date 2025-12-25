"""
Skeenode AI Service Configuration

Centralized configuration management using Pydantic settings
with environment variable support and validation.
"""

import os
from functools import lru_cache
from typing import List, Optional

from pydantic import Field
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings with environment variable binding"""
    
    # Service
    service_name: str = Field(default="skeenode-ai", env="SERVICE_NAME")
    environment: str = Field(default="development", env="ENVIRONMENT")
    debug: bool = Field(default=False, env="DEBUG")
    
    # Server
    host: str = Field(default="0.0.0.0", env="HOST")
    port: int = Field(default=8000, env="PORT")
    workers: int = Field(default=4, env="WORKERS")
    
    # Redis
    redis_url: str = Field(default="redis://localhost:6379", env="REDIS_URL")
    
    # Model
    model_path: str = Field(default="./models", env="MODEL_PATH")
    model_version: Optional[str] = Field(default=None, env="MODEL_VERSION")
    auto_reload_interval: int = Field(default=30, env="MODEL_RELOAD_INTERVAL")
    
    # Feature flags
    ab_testing_enabled: bool = Field(default=True, env="AB_TESTING_ENABLED")
    prediction_caching_enabled: bool = Field(default=True, env="PREDICTION_CACHE_ENABLED")
    cache_ttl_seconds: int = Field(default=300, env="CACHE_TTL_SECONDS")
    
    # Rate limiting
    rate_limit_enabled: bool = Field(default=True, env="RATE_LIMIT_ENABLED")
    rate_limit_requests: int = Field(default=100, env="RATE_LIMIT_REQUESTS")
    rate_limit_window: int = Field(default=60, env="RATE_LIMIT_WINDOW")
    
    # Tracing
    otel_enabled: bool = Field(default=False, env="OTEL_ENABLED")
    otel_endpoint: str = Field(default="localhost:4318", env="OTEL_ENDPOINT")
    otel_sampling_rate: float = Field(default=1.0, env="OTEL_SAMPLING_RATE")
    
    # Logging
    log_level: str = Field(default="INFO", env="LOG_LEVEL")
    log_format: str = Field(default="json", env="LOG_FORMAT")  # json or text
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False


@lru_cache()
def get_settings() -> Settings:
    """Get cached settings instance"""
    return Settings()


# Convenience access
settings = get_settings()
