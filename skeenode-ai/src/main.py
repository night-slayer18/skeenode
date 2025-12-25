from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import uvicorn
import os
import pandas as pd
import numpy as np
from xgboost import XGBClassifier
import joblib
import logging
from typing import Dict, Any

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Skeenode Intelligence Service")

# Global model variable
model = None

class PredictionRequest(BaseModel):
    job_id: str
    features: Dict[str, Any]

class PredictionResponse(BaseModel):
    job_id: str
    failure_probability: float
    confidence: float
    decision: str

@app.on_event("startup")
def load_or_train_model():
    global model
    model_path = "model.joblib"
    if os.path.exists(model_path):
        logger.info("Loading existing model...")
        model = joblib.load(model_path)
    else:
        logger.info("Training initial dummy model...")
        # Train a dummy model
        # Features: day_of_week (0-6), hour (0-23), job_type_encoded
        # Target: 0 (Success), 1 (Failure)

        # Create synthetic data
        n_samples = 1000
        X = pd.DataFrame({
            'day_of_week': np.random.randint(0, 7, n_samples),
            'hour': np.random.randint(0, 24, n_samples),
            'job_type_len': np.random.randint(4, 10, n_samples) # Mock encoding
        })

        # Simulate some logic: weekends have higher failure rate, late night has higher failure rate
        y = []
        for _, row in X.iterrows():
            prob = 0.1
            if row['day_of_week'] >= 5: # Weekend
                prob += 0.3
            if row['hour'] < 6: # Late night
                prob += 0.2
            y.append(1 if np.random.random() < prob else 0)

        model = XGBClassifier(n_estimators=100, max_depth=3, eval_metric='logloss')
        model.fit(X, y)
        joblib.dump(model, model_path)
        logger.info("Model trained and saved.")

@app.get("/health")
def health_check():
    return {"status": "healthy", "service": "skeenode-ai"}

@app.post("/predict/failure", response_model=PredictionResponse)
def predict_failure(req: PredictionRequest):
    global model
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")

    try:
        # Preprocess features
        # Expecting features like: {"day_of_week": 1, "hour": 10, "job_type": "SHELL"}

        # Mock encoding for job_type since we don't have a real encoder persisted yet
        job_type = req.features.get("job_type", "SHELL")
        job_type_len = len(job_type)

        features_df = pd.DataFrame([{
            'day_of_week': req.features.get('day_of_week', 0),
            'hour': req.features.get('hour', 12),
            'job_type_len': job_type_len
        }])

        # Predict probability of class 1 (Failure)
        prob_fail = model.predict_proba(features_df)[0][1]

        decision = "PROCEED"
        if prob_fail > 0.7:
            decision = "ABORT"

        logger.info(f"Prediction for job {req.job_id}: ProbFail={prob_fail:.4f}, Decision={decision}")

        return {
            "job_id": req.job_id,
            "failure_probability": float(prob_fail),
            "confidence": float(1.0), # Simplification
            "decision": decision
        }
    except Exception as e:
        logger.error(f"Prediction error: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)
