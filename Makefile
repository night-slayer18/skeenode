# Skeenode Master Makefile

.PHONY: all build clean test run-backend run-frontend run-ai

all: build

build: build-backend build-frontend

build-backend:
	@echo "Building Backend..."
	cd skeenode-backend && go build -o ../bin/skeenode-scheduler ./cmd/scheduler
	cd skeenode-backend && go build -o ../bin/skeenode-executor ./cmd/executor

build-frontend:
	@echo "Building Frontend..."
	cd skeenode-frontend && npm install && npm run build

setup:
	@echo "Setting up environment..."
	cd skeenode-backend && go mod download
	cd skeenode-ai && pip install -r requirements.txt
	cd skeenode-frontend && npm install

up:
	docker-compose up -d

down:
	docker-compose down

test:
	cd skeenode-backend && go test ./...
