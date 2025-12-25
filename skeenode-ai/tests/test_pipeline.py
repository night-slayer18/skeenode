import unittest
import pandas as pd
import os
import shutil
import sys

# Add src to path
sys.path.append(os.path.join(os.path.dirname(__file__), '../src'))

from pipeline import TrainingPipeline

class TestPipeline(unittest.TestCase):
    def setUp(self):
        self.test_model_path = "test_model.joblib"
        self.pipeline = TrainingPipeline(model_path=self.test_model_path)

    def tearDown(self):
        if os.path.exists(self.test_model_path):
            os.remove(self.test_model_path)

    def test_cold_start_training(self):
        # Should have trained a model on init because file didn't exist
        self.assertIsNotNone(self.pipeline.model)
        self.assertTrue(os.path.exists(self.test_model_path))

    def test_prediction(self):
        features = pd.DataFrame([{
            'day_of_week': 0,
            'hour': 12,
            'job_type_len': 5
        }])
        prob = self.pipeline.predict(features)
        self.assertTrue(0.0 <= prob <= 1.0)

    def test_retrain(self):
        old_model = self.pipeline.model
        self.pipeline.retrain()
        new_model = self.pipeline.model
        # Models should be different objects (though logic is deterministic in dummy, pointers differ)
        self.assertNotEqual(old_model, new_model)

if __name__ == '__main__':
    unittest.main()
