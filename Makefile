# Makefile
.PHONY: up down reset logs db build_proto deps test run build server stop-server logs-server clean

COMPOSE = docker-compose

# Database Management Commands

up:
	@echo "Starting MySQL container..."
	$(COMPOSE) up -d mysql

down:
	@echo "Stopping all containers..."
	$(COMPOSE) down

reset:
	@echo "Resetting all containers and deleting data..."
	$(COMPOSE) down -v
	$(COMPOSE) up -d mysql

logs:
	@echo "Showing MySQL logs (Ctrl+C to exit)..."
	$(COMPOSE) logs -f mysql

db:
	@echo "Opening MySQL CLI (root user)(\q to exit)..."
	docker exec -it my_mysql_db mysql -u root -p myapp_db

# Build and Dependency Management

build_proto:
	@echo "Generating Go proto files..."
	protoc --go_out=./explore_service_proto --go_opt=paths=source_relative \
        --go-grpc_out=./explore_service_proto --go-grpc_opt=paths=source_relative \
        explore-service.proto

deps: build_proto
	@echo "Downloading Go dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed successfully!"

# Testing

test: deps
	@echo "Running tests..."
	go test -v ./internal/...

# Local Development (without Docker)

run: deps
	@echo "Starting server locally (requires MySQL to be running)..."
	@echo "Make sure MySQL is running with: make up"
	go run ./cmd/main.go

# Docker-based Server Management

build: build_proto
	@echo "Building Docker image..."
	$(COMPOSE) build server
	@echo "Docker image built successfully!"

server: up build
	@echo "Starting server container..."
	$(COMPOSE) up -d server
	@echo "Server is running on port 9001"
	@echo "View logs with: make logs-server"
	@echo "Stop server with: make stop-server"

stop-server:
	@echo "Stopping server container..."
	$(COMPOSE) stop server

logs-server:
	@echo "Showing server logs (Ctrl+C to exit)..."
	$(COMPOSE) logs -f server

# Utility Commands

clean:
	@echo "Cleaning up..."
	$(COMPOSE) down -v
	docker system prune -f
	@echo "Cleanup complete!"

