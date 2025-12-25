"""
Model Training Pipeline

Handles training workflows, hyperparameter tuning,
and model evaluation with proper metrics.
"""

import json
import logging
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import joblib
import numpy as np
import pandas as pd
from sklearn.metrics import (
    accuracy_score,
    f1_score,
    precision_score,
    recall_score,
    roc_auc_score,
)
from sklearn.model_selection import cross_val_score, train_test_split
from xgboost import XGBClassifier

from ..config import settings
from ..model_registry import ModelRegistry, get_registry
from .collector import TrainingDataCollector, TrainingDataConfig

logger = logging.getLogger(__name__)


@dataclass
class TrainingConfig:
    """Training pipeline configuration"""
    
    # Data
    database_url: str = ""
    lookback_days: int = 90
    test_size: float = 0.2
    
    # Model hyperparameters
    n_estimators: int = 100
    max_depth: int = 5
    learning_rate: float = 0.1
    min_child_weight: int = 1
    subsample: float = 0.8
    colsample_bytree: float = 0.8
    
    # Training
    early_stopping_rounds: int = 10
    n_cv_folds: int = 5
    
    # Output
    model_path: str = "./models"
    activate_on_success: bool = False
    min_accuracy: float = 0.7  # Minimum accuracy to register model


@dataclass
class TrainingResult:
    """Results from a training run"""
    
    success: bool
    version_id: Optional[str] = None
    metrics: Dict[str, float] = field(default_factory=dict)
    training_time_seconds: float = 0.0
    samples_used: int = 0
    features_used: List[str] = field(default_factory=list)
    error: Optional[str] = None


class TrainingPipeline:
    """
    End-to-end model training pipeline.
    
    Steps:
    1. Collect data from PostgreSQL
    2. Split into train/test sets
    3. Train XGBoost classifier
    4. Evaluate on held-out test set
    5. Register model if performance is acceptable
    """
    
    def __init__(
        self,
        config: Optional[TrainingConfig] = None,
        registry: Optional[ModelRegistry] = None,
    ):
        self.config = config or TrainingConfig()
        self.registry = registry or get_registry()
    
    def train(self) -> TrainingResult:
        """
        Execute the full training pipeline.
        
        Returns:
            TrainingResult with metrics and model version
        """
        start_time = time.time()
        
        try:
            # Step 1: Collect data
            logger.info("Step 1/5: Collecting training data...")
            X, y = self._collect_data()
            
            if len(X) == 0:
                return TrainingResult(
                    success=False,
                    error="No training data available",
                )
            
            # Step 2: Split data
            logger.info("Step 2/5: Splitting train/test sets...")
            X_train, X_test, y_train, y_test = train_test_split(
                X, y,
                test_size=self.config.test_size,
                random_state=42,
                stratify=y,
            )
            
            # Step 3: Train model
            logger.info("Step 3/5: Training XGBoost model...")
            model = self._train_model(X_train, y_train)
            
            # Step 4: Evaluate
            logger.info("Step 4/5: Evaluating model...")
            metrics = self._evaluate_model(model, X_test, y_test)
            
            # Check minimum accuracy
            if metrics["accuracy"] < self.config.min_accuracy:
                return TrainingResult(
                    success=False,
                    metrics=metrics,
                    samples_used=len(X),
                    features_used=list(X.columns),
                    training_time_seconds=time.time() - start_time,
                    error=f"Model accuracy {metrics['accuracy']:.3f} below threshold {self.config.min_accuracy}",
                )
            
            # Step 5: Register model
            logger.info("Step 5/5: Registering model...")
            version_id = self._register_model(model, metrics, list(X.columns))
            
            return TrainingResult(
                success=True,
                version_id=version_id,
                metrics=metrics,
                samples_used=len(X),
                features_used=list(X.columns),
                training_time_seconds=time.time() - start_time,
            )
        
        except Exception as e:
            logger.error(f"Training failed: {e}", exc_info=True)
            return TrainingResult(
                success=False,
                error=str(e),
                training_time_seconds=time.time() - start_time,
            )
    
    def _collect_data(self) -> Tuple[pd.DataFrame, pd.Series]:
        """Collect training data"""
        data_config = TrainingDataConfig(
            database_url=self.config.database_url,
            lookback_days=self.config.lookback_days,
        )
        collector = TrainingDataCollector(data_config)
        return collector.collect()
    
    def _train_model(
        self,
        X_train: pd.DataFrame,
        y_train: pd.Series,
    ) -> XGBClassifier:
        """Train XGBoost model"""
        model = XGBClassifier(
            n_estimators=self.config.n_estimators,
            max_depth=self.config.max_depth,
            learning_rate=self.config.learning_rate,
            min_child_weight=self.config.min_child_weight,
            subsample=self.config.subsample,
            colsample_bytree=self.config.colsample_bytree,
            eval_metric="logloss",
            use_label_encoder=False,
            random_state=42,
        )
        
        # Cross-validation for stability check
        cv_scores = cross_val_score(
            model, X_train, y_train,
            cv=self.config.n_cv_folds,
            scoring="accuracy",
        )
        logger.info(f"CV Accuracy: {cv_scores.mean():.3f} (+/- {cv_scores.std()*2:.3f})")
        
        # Final training
        model.fit(X_train, y_train)
        
        return model
    
    def _evaluate_model(
        self,
        model: XGBClassifier,
        X_test: pd.DataFrame,
        y_test: pd.Series,
    ) -> Dict[str, float]:
        """Evaluate model on test set"""
        y_pred = model.predict(X_test)
        y_proba = model.predict_proba(X_test)[:, 1]
        
        metrics = {
            "accuracy": accuracy_score(y_test, y_pred),
            "precision": precision_score(y_test, y_pred, zero_division=0),
            "recall": recall_score(y_test, y_pred, zero_division=0),
            "f1": f1_score(y_test, y_pred, zero_division=0),
            "roc_auc": roc_auc_score(y_test, y_proba) if y_test.nunique() > 1 else 0.0,
        }
        
        logger.info(f"Test metrics: {json.dumps(metrics, indent=2)}")
        
        return metrics
    
    def _register_model(
        self,
        model: XGBClassifier,
        metrics: Dict[str, float],
        features: List[str],
    ) -> str:
        """Register trained model with registry"""
        version_id = self.registry.register_model(
            model=model,
            metrics=metrics,
            features=features,
            model_type="xgboost",
            activate=self.config.activate_on_success,
        )
        
        logger.info(f"Registered model version: {version_id}")
        return version_id


def run_training(
    database_url: str,
    activate: bool = False,
) -> TrainingResult:
    """
    Convenience function to run training.
    
    Args:
        database_url: PostgreSQL connection string
        activate: Whether to activate model on success
    
    Returns:
        TrainingResult
    """
    config = TrainingConfig(
        database_url=database_url,
        activate_on_success=activate,
    )
    pipeline = TrainingPipeline(config)
    return pipeline.train()
