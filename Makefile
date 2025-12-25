# Skeenode Master Makefile

.PHONY: all build clean test run-backend run-frontend run-ai lint docker-build

# Variables
BACKEND_DIR=skeenode-backend
FRONTEND_DIR=skeenode-frontend
AI_DIR=skeenode-ai
BIN_DIR=bin

# Default target
all: build

# Setup dependencies
setup:
	@echo "🔧 Setting up environment..."
	cd $(BACKEND_DIR) && go mod download
	cd $(AI_DIR) && pip install -r requirements.txt
	cd $(FRONTEND_DIR) && npm install

# Build all services
build: build-backend build-frontend

build-backend:
	@echo "🐘 Building Backend..."
	mkdir -p $(BIN_DIR)
	cd $(BACKEND_DIR) && go build -o ../$(BIN_DIR)/skeenode-scheduler ./cmd/scheduler
	cd $(BACKEND_DIR) && go build -o ../$(BIN_DIR)/skeenode-executor ./cmd/executor
	cd $(BACKEND_DIR) && go build -o ../$(BIN_DIR)/skeenode-api ./cmd/api

build-frontend:
	@echo "⚛️ Building Frontend..."
	cd $(FRONTEND_DIR) && npm run build

# Testing
test: test-backend test-frontend test-ai

test-backend:
	@echo "🧪 Testing Backend..."
	cd $(BACKEND_DIR) && go test -v ./...

test-frontend:
	@echo "🧪 Testing Frontend (Linting)..."
	cd $(FRONTEND_DIR) && npm run lint

test-ai:
	@echo "🧪 Testing AI..."
	cd $(AI_DIR) && python3 -m pytest

# Linting
lint:
	cd $(FRONTEND_DIR) && npm run lint
	cd $(BACKEND_DIR) && go vet ./...

# Running Local (Dev)
run-backend:
	@echo "🚀 Running Backend Scheduler..."
	./$(BIN_DIR)/skeenode-scheduler

run-frontend:
	@echo "🚀 Running Frontend..."
	cd $(FRONTEND_DIR) && npm run dev

run-ai:
	@echo "🧠 Running AI Service..."
	cd $(AI_DIR) && uvicorn main:app --reload

# Docker
docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down

# Clean
clean:
	rm -rf $(BIN_DIR)
	cd $(FRONTEND_DIR) && rm -rf dist
