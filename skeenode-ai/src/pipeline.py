import pandas as pd
import numpy as np
from xgboost import XGBClassifier
import joblib
import logging
import os
from typing import Optional

logger = logging.getLogger(__name__)

class TrainingPipeline:
    def __init__(self, model_path: str = "model.joblib"):
        self.model_path = model_path
        self.model: Optional[XGBClassifier] = None
        self.load_model()

    def load_model(self):
        if os.path.exists(self.model_path):
            try:
                self.model = joblib.load(self.model_path)
                logger.info(f"Loaded model from {self.model_path}")
            except Exception as e:
                logger.error(f"Failed to load model: {e}")
        else:
            logger.info("No existing model found. Using cold-start dummy model.")
            self.train_dummy_model()

    def train_dummy_model(self):
        logger.info("Training initial dummy model...")
        # Create synthetic data
        n_samples = 1000
        X = pd.DataFrame({
            'day_of_week': np.random.randint(0, 7, n_samples),
            'hour': np.random.randint(0, 24, n_samples),
            'job_type_len': np.random.randint(4, 10, n_samples)
        })

        y = []
        for _, row in X.iterrows():
            prob = 0.1
            if row['day_of_week'] >= 5: # Weekend
                prob += 0.3
            if row['hour'] < 6: # Late night
                prob += 0.2
            y.append(1 if np.random.random() < prob else 0)

        self.train(X, y)

    def train(self, X: pd.DataFrame, y: list):
        model = XGBClassifier(n_estimators=100, max_depth=3, eval_metric='logloss')
        model.fit(X, y)
        self.model = model

        # Persist
        joblib.dump(self.model, self.model_path)
        logger.info(f"Model trained and saved to {self.model_path}")

    def predict(self, features: pd.DataFrame) -> float:
        if self.model is None:
            # Fallback if training failed or corrupted
            return 0.0
        return float(self.model.predict_proba(features)[0][1])

    def retrain(self):
        # In a real system, this would query a Feature Store or DB for fresh data.
        # For now, we simulate finding new data and retraining.
        logger.info("Retraining triggered...")
        self.train_dummy_model() # Simulate retraining with new random data
