"""
Rate Limiting Middleware

Token bucket rate limiting with per-client tracking.
"""

import time
from collections import defaultdict
from dataclasses import dataclass
from threading import Lock
from typing import Dict, Optional

from fastapi import HTTPException, Request
from starlette.middleware.base import BaseHTTPMiddleware

from ..config import settings


@dataclass
class TokenBucket:
    """Token bucket for rate limiting"""
    tokens: float
    last_refill: float
    rate: float  # tokens per second
    capacity: int


class RateLimiter:
    """Thread-safe rate limiter with per-client tracking"""
    
    def __init__(
        self,
        requests_per_window: int = 100,
        window_seconds: int = 60,
    ):
        self.rate = requests_per_window / window_seconds
        self.capacity = requests_per_window
        self.buckets: Dict[str, TokenBucket] = {}
        self.lock = Lock()
    
    def allow(self, client_id: str) -> bool:
        """Check if request is allowed"""
        with self.lock:
            now = time.time()
            
            if client_id not in self.buckets:
                self.buckets[client_id] = TokenBucket(
                    tokens=self.capacity,
                    last_refill=now,
                    rate=self.rate,
                    capacity=self.capacity,
                )
            
            bucket = self.buckets[client_id]
            
            # Refill tokens
            elapsed = now - bucket.last_refill
            bucket.tokens = min(
                bucket.capacity,
                bucket.tokens + elapsed * bucket.rate,
            )
            bucket.last_refill = now
            
            # Check if allowed
            if bucket.tokens >= 1:
                bucket.tokens -= 1
                return True
            
            return False
    
    def get_retry_after(self, client_id: str) -> int:
        """Get seconds until next token available"""
        with self.lock:
            bucket = self.buckets.get(client_id)
            if bucket is None or bucket.tokens >= 1:
                return 0
            return int((1 - bucket.tokens) / bucket.rate) + 1


class RateLimitMiddleware(BaseHTTPMiddleware):
    """FastAPI middleware for rate limiting"""
    
    def __init__(self, app, limiter: Optional[RateLimiter] = None):
        super().__init__(app)
        self.limiter = limiter or RateLimiter(
            requests_per_window=settings.rate_limit_requests,
            window_seconds=settings.rate_limit_window,
        )
        self.enabled = settings.rate_limit_enabled
    
    async def dispatch(self, request: Request, call_next):
        if not self.enabled:
            return await call_next(request)
        
        # Skip health endpoints
        if request.url.path in {"/health", "/ready", "/live"}:
            return await call_next(request)
        
        # Get client ID (prefer X-Forwarded-For, fallback to IP)
        client_id = request.headers.get(
            "X-Forwarded-For",
            request.client.host if request.client else "unknown",
        )
        
        if not self.limiter.allow(client_id):
            retry_after = self.limiter.get_retry_after(client_id)
            raise HTTPException(
                status_code=429,
                detail="Rate limit exceeded",
                headers={"Retry-After": str(retry_after)},
            )
        
        return await call_next(request)
