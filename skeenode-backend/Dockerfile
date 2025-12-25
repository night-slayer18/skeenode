# Backend Builder
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN go build -o /app/bin/scheduler ./cmd/scheduler/main.go
RUN go build -o /app/bin/executor ./cmd/executor/main.go
RUN go build -o /app/bin/api ./cmd/api/main.go

# Runtime Image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies (e.g. bash for debugging)
RUN apk add --no-cache bash ca-certificates

# Copy binaries from builder
COPY --from=builder /app/bin/scheduler /app/scheduler
COPY --from=builder /app/bin/executor /app/executor
COPY --from=builder /app/bin/api /app/api

# Default to running api, can be overridden
CMD ["/app/api"]
