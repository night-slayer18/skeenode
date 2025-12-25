"""
Training Data Collection and Feature Engineering

Collects historical job execution data from PostgreSQL and
transforms it into features for model training.
"""

import logging
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import pandas as pd
from sqlalchemy import create_engine, text
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)


@dataclass
class TrainingDataConfig:
    """Configuration for training data collection"""
    
    # Database
    database_url: str = field(default_factory=lambda: "")
    
    # Time range
    lookback_days: int = 90  # How far back to look for training data
    min_samples: int = 1000  # Minimum samples required
    
    # Feature engineering
    include_time_features: bool = True
    include_job_features: bool = True
    include_historical_features: bool = True
    
    # Sampling
    sample_ratio: float = 1.0  # 1.0 = use all data
    balance_classes: bool = True  # Balance success/failure classes


class TrainingDataCollector:
    """
    Collects and transforms job execution data into ML-ready features.
    
    Data Flow:
    1. Query PostgreSQL for historical executions
    2. Join with job metadata for context
    3. Engineer features (time, historical patterns, job characteristics)
    4. Return labeled DataFrame for training
    """
    
    # SQL query for execution history
    EXECUTION_QUERY = """
    SELECT 
        e.id AS execution_id,
        e.job_id,
        e.status,
        e.scheduled_at,
        e.started_at,
        e.completed_at,
        e.exit_code,
        e.node_id,
        j.name AS job_name,
        j.type AS job_type,
        j.command,
        j.schedule,
        j.owner_id
    FROM executions e
    JOIN jobs j ON e.job_id = j.id
    WHERE e.completed_at IS NOT NULL
      AND e.completed_at >= :start_date
      AND e.status IN ('SUCCESS', 'FAILED')
    ORDER BY e.completed_at DESC
    """
    
    def __init__(
        self,
        config: Optional[TrainingDataConfig] = None,
        engine: Optional[Engine] = None,
    ):
        self.config = config or TrainingDataConfig()
        self._engine = engine
    
    @property
    def engine(self) -> Engine:
        """Lazy database connection"""
        if self._engine is None:
            db_url = self.config.database_url
            if not db_url:
                raise ValueError("database_url not configured")
            self._engine = create_engine(db_url, pool_pre_ping=True)
        return self._engine
    
    def collect(self) -> Tuple[pd.DataFrame, pd.Series]:
        """
        Collect training data and return features + labels.
        
        Returns:
            Tuple of (X: features DataFrame, y: labels Series)
        """
        logger.info("Collecting training data from database...")
        
        # Fetch raw execution data
        raw_df = self._fetch_executions()
        
        if len(raw_df) < self.config.min_samples:
            logger.warning(
                f"Only {len(raw_df)} samples found, need {self.config.min_samples}. "
                "Using synthetic augmentation."
            )
            raw_df = self._augment_with_synthetic(raw_df)
        
        logger.info(f"Collected {len(raw_df)} execution records")
        
        # Engineer features
        features_df = self._engineer_features(raw_df)
        
        # Create labels (1 = failure, 0 = success)
        labels = (raw_df["status"] == "FAILED").astype(int)
        
        # Balance classes if configured
        if self.config.balance_classes:
            features_df, labels = self._balance_classes(features_df, labels)
        
        # Sample if ratio < 1
        if self.config.sample_ratio < 1.0:
            n_samples = int(len(features_df) * self.config.sample_ratio)
            indices = np.random.choice(len(features_df), n_samples, replace=False)
            features_df = features_df.iloc[indices]
            labels = labels.iloc[indices]
        
        logger.info(
            f"Final dataset: {len(features_df)} samples, "
            f"{labels.sum()} failures ({labels.mean():.1%} rate)"
        )
        
        return features_df, labels
    
    def _fetch_executions(self) -> pd.DataFrame:
        """Fetch execution history from PostgreSQL"""
        start_date = datetime.utcnow() - timedelta(days=self.config.lookback_days)
        
        with self.engine.connect() as conn:
            result = conn.execute(
                text(self.EXECUTION_QUERY),
                {"start_date": start_date},
            )
            df = pd.DataFrame(result.fetchall(), columns=result.keys())
        
        return df
    
    def _engineer_features(self, df: pd.DataFrame) -> pd.DataFrame:
        """Transform raw data into ML features"""
        features = pd.DataFrame(index=df.index)
        
        # Time-based features
        if self.config.include_time_features:
            features["day_of_week"] = pd.to_datetime(df["scheduled_at"]).dt.dayofweek
            features["hour"] = pd.to_datetime(df["scheduled_at"]).dt.hour
            features["is_weekend"] = features["day_of_week"] >= 5
            features["is_night"] = (features["hour"] < 6) | (features["hour"] >= 22)
            features["is_business_hours"] = (features["hour"] >= 9) & (features["hour"] < 17)
        
        # Job characteristics
        if self.config.include_job_features:
            features["job_type_encoded"] = df["job_type"].factorize()[0]
            features["command_length"] = df["command"].str.len()
            features["has_schedule"] = df["schedule"].notna().astype(int)
        
        # Historical features (per-job statistics)
        if self.config.include_historical_features:
            job_stats = self._compute_job_statistics(df)
            features = features.merge(
                job_stats,
                left_on=df["job_id"],
                right_index=True,
                how="left",
            ).fillna(0)
            # Drop the merge key if added
            if "key_0" in features.columns:
                features = features.drop(columns=["key_0"])
        
        return features
    
    def _compute_job_statistics(self, df: pd.DataFrame) -> pd.DataFrame:
        """Compute per-job historical statistics"""
        job_stats = df.groupby("job_id").agg({
            "execution_id": "count",
            "status": lambda x: (x == "FAILED").mean(),
        }).rename(columns={
            "execution_id": "execution_count",
            "status": "historical_failure_rate",
        })
        
        # Compute average duration for successful executions
        successful = df[df["status"] == "SUCCESS"].copy()
        if len(successful) > 0 and "started_at" in successful.columns:
            successful["started_at"] = pd.to_datetime(successful["started_at"])
            successful["completed_at"] = pd.to_datetime(successful["completed_at"])
            successful["duration_ms"] = (
                (successful["completed_at"] - successful["started_at"])
                .dt.total_seconds() * 1000
            )
            duration_stats = successful.groupby("job_id")["duration_ms"].mean()
            job_stats["avg_duration_ms"] = duration_stats
        
        return job_stats
    
    def _balance_classes(
        self, X: pd.DataFrame, y: pd.Series
    ) -> Tuple[pd.DataFrame, pd.Series]:
        """Balance success/failure classes via undersampling"""
        n_failures = y.sum()
        n_successes = len(y) - n_failures
        
        if n_failures == 0 or n_successes == 0:
            return X, y
        
        # Undersample majority class
        minority_size = min(n_failures, n_successes)
        
        failure_indices = y[y == 1].index
        success_indices = y[y == 0].index
        
        if n_failures > n_successes:
            failure_indices = np.random.choice(failure_indices, minority_size, replace=False)
        else:
            success_indices = np.random.choice(success_indices, minority_size, replace=False)
        
        indices = np.concatenate([failure_indices, success_indices])
        np.random.shuffle(indices)
        
        return X.loc[indices], y.loc[indices]
    
    def _augment_with_synthetic(self, df: pd.DataFrame) -> pd.DataFrame:
        """Augment with synthetic data when real data is insufficient"""
        n_needed = self.config.min_samples - len(df)
        
        logger.info(f"Generating {n_needed} synthetic training samples")
        
        # Generate synthetic executions
        synthetic = pd.DataFrame({
            "execution_id": [f"synthetic_{i}" for i in range(n_needed)],
            "job_id": [f"synthetic_job_{i % 10}" for i in range(n_needed)],
            "status": np.random.choice(["SUCCESS", "FAILED"], n_needed, p=[0.85, 0.15]),
            "scheduled_at": [
                datetime.utcnow() - timedelta(hours=np.random.randint(1, 2000))
                for _ in range(n_needed)
            ],
            "started_at": None,
            "completed_at": None,
            "exit_code": 0,
            "node_id": "synthetic-node",
            "job_name": "synthetic-job",
            "job_type": np.random.choice(["SHELL", "DOCKER", "HTTP"], n_needed),
            "command": "echo synthetic",
            "schedule": "*/5 * * * *",
            "owner_id": "synthetic-owner",
        })
        
        return pd.concat([df, synthetic], ignore_index=True)


def get_training_data(
    database_url: str,
    lookback_days: int = 90,
) -> Tuple[pd.DataFrame, pd.Series]:
    """
    Convenience function to collect training data.
    
    Args:
        database_url: PostgreSQL connection string
        lookback_days: Days of history to include
    
    Returns:
        Tuple of (features, labels)
    """
    config = TrainingDataConfig(
        database_url=database_url,
        lookback_days=lookback_days,
    )
    collector = TrainingDataCollector(config)
    return collector.collect()
