FRONTEND_DIR := web
BACKEND_ENTRY := ./cmd/server
BACKEND_BIN_DIR := .
BACKEND_BIN := $(BACKEND_BIN_DIR)/server

.PHONY: help deps deps-frontend deps-backend build build-frontend build-backend clean

help:
	@echo "Available targets:"
	@echo "  make deps            Install frontend and backend dependencies"
	@echo "  make build-frontend  Build frontend assets (web/dist)"
	@echo "  make build-backend   Build backend binary (./server)"
	@echo "  make build           Build frontend and backend"
	@echo "  make clean           Remove build artifacts"

deps: deps-frontend deps-backend

deps-frontend:
	cd $(FRONTEND_DIR) && bun install

deps-backend:
	go mod download

build: build-backend

build-frontend:
	cd $(FRONTEND_DIR) && bun run build

build-backend: build-frontend
	mkdir -p $(BACKEND_BIN_DIR)
	go build -o $(BACKEND_BIN) $(BACKEND_ENTRY)

clean:
	rm -rf $(FRONTEND_DIR)/dist $(BACKEND_BIN_DIR)
