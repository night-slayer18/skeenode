from fastapi import FastAPI, HTTPException, BackgroundTasks
from pydantic import BaseModel
import uvicorn
import os
import pandas as pd
import logging
from typing import Dict, Any

from pipeline import TrainingPipeline

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Skeenode Intelligence Service")

# Initialize Pipeline (loads or creates model)
# We store model in /data volume for persistence in K8s
MODEL_PATH = os.getenv("MODEL_PATH", "/data/model.joblib")
if not os.path.exists(os.path.dirname(MODEL_PATH)):
    os.makedirs(os.path.dirname(MODEL_PATH), exist_ok=True)

pipeline = TrainingPipeline(model_path=MODEL_PATH)

class PredictionRequest(BaseModel):
    job_id: str
    features: Dict[str, Any]

class PredictionResponse(BaseModel):
    job_id: str
    failure_probability: float
    confidence: float
    decision: str

@app.get("/health")
def health_check():
    status = "healthy" if pipeline.model is not None else "degraded"
    return {"status": status, "service": "skeenode-ai"}

@app.post("/predict/failure", response_model=PredictionResponse)
def predict_failure(req: PredictionRequest):
    try:
        # Preprocess features
        job_type = req.features.get("job_type", "SHELL")
        job_type_len = len(job_type)

        features_df = pd.DataFrame([{
            'day_of_week': req.features.get('day_of_week', 0),
            'hour': req.features.get('hour', 12),
            'job_type_len': job_type_len
        }])

        prob_fail = pipeline.predict(features_df)

        decision = "PROCEED"
        # Threshold could be dynamic based on job priority
        if prob_fail > 0.8:
            decision = "ABORT"

        return {
            "job_id": req.job_id,
            "failure_probability": prob_fail,
            "confidence": 1.0,
            "decision": decision
        }
    except Exception as e:
        logger.error(f"Prediction error: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/retrain")
def trigger_retrain(background_tasks: BackgroundTasks):
    background_tasks.add_task(pipeline.retrain)
    return {"status": "retraining_started"}

if __name__ == "__main__":
    port = int(os.getenv("PORT", 8000))
    uvicorn.run(app, host="0.0.0.0", port=port)
