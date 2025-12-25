"""
Unit Tests for Configuration
"""

import os
import pytest


class TestConfig:
    """Test configuration loading"""
    
    def test_default_values(self):
        """Test default configuration values"""
        # Import with fresh environment
        from src.config import Settings
        
        settings = Settings()
        assert settings.service_name == "skeenode-ai"
        assert settings.port == 8000
        assert settings.log_level == "INFO"
    
    def test_environment_override(self, monkeypatch):
        """Test environment variables override defaults"""
        monkeypatch.setenv("PORT", "9000")
        monkeypatch.setenv("LOG_LEVEL", "DEBUG")
        
        from src.config import Settings
        settings = Settings()
        
        assert settings.port == 9000
        assert settings.log_level == "DEBUG"
    
    def test_rate_limit_defaults(self):
        """Test rate limiting defaults"""
        from src.config import Settings
        
        settings = Settings()
        assert settings.rate_limit_enabled is True
        assert settings.rate_limit_requests == 100
        assert settings.rate_limit_window == 60
