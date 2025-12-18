# Skeenode
> **Intelligent Distributed Job Scheduler**

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21-cyan.svg)](skeenode-backend/go.mod)
[![Status](https://img.shields.io/badge/status-active-success.svg)]()

Skeenode (formerly ChronoFlow) is a next-generation distributed job scheduler designed to supersede legacy systems like Cron and Rundeck. It leverages **Machine Learning** to predict job failures, optimize execution windows, and manage dependencies automatically.

## 🚀 Features

*   **Smart Scheduling**: Dynamic execution windows based on real-time cluster load.
*   **Predictive Failure**: ML models analyze history to prevent doomed executions.
*   **Self-Healing**: Leader election and automatic failover via etcd.
*   **Zero-Config**: Auto-discovery of job dependencies.

## 📂 Project Structure

*   `skeenode-backend/`: Core Scheduler & Executor (Go)
*   `skeenode-frontend/`: Admin Dashboard (React/Vue)
*   `skeenode-ai/`: Intelligence Service (Python)
*   `infrastructure/`: Docker & K8s Manifests
*   `docs/`: Design Documents & Blueprints

## 🛠️ Quick Start

### Prerequisites
*   Docker & Docker Compose
*   Go 1.21+
*   Node.js 18+

### Running Locally
```bash
# Start infrastructure (Postgres, Redis, Etcd)
make up

# Build all services
make build

# Run Backend
cd skeenode-backend && go run cmd/scheduler/main.go
```

## 📖 Documentation
See the [docs/](docs/) directory for detailed architecture and implementation plans.

## 🤝 Contributing
Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 📄 License
This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.