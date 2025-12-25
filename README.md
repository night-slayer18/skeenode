# Skeenode
> **Intelligent Distributed Job Scheduler**

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24-cyan.svg)](skeenode-backend/go.mod)
[![Status](https://img.shields.io/badge/status-active-success.svg)]()
[![Kubernetes](https://img.shields.io/badge/kubernetes-ready-blue.svg)](k8s/)

Skeenode is a next-generation distributed job scheduler designed to supersede legacy systems like Cron and Rundeck. It leverages **Machine Learning** to predict job failures, optimize execution windows, and manage dependencies automatically.

## 🚀 Features

*   **Smart Scheduling**: Dynamic execution windows based on real-time cluster load.
*   **Predictive Failure**: AI models (XGBoost) analyze history to prevent doomed executions.
*   **Self-Healing**: Leader election and automatic failover via etcd.
*   **Scalable Architecture**: Microservices design deployable on Kubernetes with Horizontal Pod Autoscaling (HPA).
*   **User Dashboard**: React-based UI for managing jobs and viewing execution history.

## 📂 Project Structure

*   `skeenode-backend/`: Core services (Scheduler, Executor, API) written in Go.
*   `skeenode-ai/`: Intelligence Service (Python/FastAPI + XGBoost).
*   `skeenode-frontend/`: Admin Dashboard (React/Vite).
*   `k8s/`: Kubernetes Manifests (Deployments, Services, HPA).
*   `docs/`: Design Documents & Blueprints.

## 🛠️ Quick Start

### Prerequisites
*   Docker & Docker Compose
*   (Optional) Kubernetes Cluster (Minikube/Kind)

### Running Locally (Docker Compose)
The easiest way to start the full stack including AI and Dashboard:

```bash
# Start all services
docker-compose up --build -d

# Access Dashboard
open http://localhost:3000
```

### Deploying to Kubernetes
Skeenode is designed for scale. To deploy to a cluster:

1. **Apply Configuration & Secrets**
   ```bash
   kubectl apply -f k8s/configmap.yaml
   kubectl apply -f k8s/secrets.yaml.example # Rename/Edit for production
   ```

2. **Deploy Services**
   ```bash
   kubectl apply -f k8s/services.yaml
   kubectl apply -f k8s/deployments-backend.yaml
   kubectl apply -f k8s/deployment-ai.yaml
   kubectl apply -f k8s/deployment-dashboard.yaml
   ```

3. **Enable Autoscaling**
   ```bash
   kubectl apply -f k8s/hpa.yaml
   ```

## 🧠 AI Module
The `skeenode-ai` service provides failure prediction.
- **Endpoint**: `POST /predict/failure`
- **Training**: Auto-trains a dummy model on startup if no model exists. Persists model to `/data/model.joblib`.
- **Retraining**: Trigger via `POST /retrain`.

## 📖 Documentation
See the [docs/](docs/) directory for detailed architecture and implementation plans.

## 🤝 Contributing
Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 📄 License
This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
