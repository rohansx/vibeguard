.PHONY: setup dev build up down logs db-migrate db-rollback db-status db-new help clean

SHELL := /bin/bash
export PATH := $(HOME)/.local/share/mise/shims:$(PATH)

COMPOSE := podman compose -f docker/docker-compose.yml
DATABASE_URL ?= postgres://vibeguard:vibeguard@localhost:5432/vibeguard?sslmode=disable
DBMATE := DATABASE_URL="$(DATABASE_URL)" dbmate -d ./apps/vgx/db/migrations -s ./apps/vgx/db/schema.sql

# Default
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development
setup: ## First-time setup: build images, start db, run migrations
	$(COMPOSE) build
	$(COMPOSE) up vibeguard-db -d
	@echo "Waiting for PostgreSQL..." && sleep 6
	$(DBMATE) up
	@echo "Ready! Run 'make up' to start all services."

dev: ## Start all services with live rebuild
	$(COMPOSE) up --build

up: ## Start all services in background
	$(COMPOSE) up -d --build

down: ## Stop all services
	$(COMPOSE) down

logs: ## Tail logs from all services
	$(COMPOSE) logs -f

build: ## Build all container images
	$(COMPOSE) build

clean: ## Stop services and remove volumes
	$(COMPOSE) down -v

# Database
db-migrate: ## Run pending migrations
	$(DBMATE) up

db-rollback: ## Rollback last migration
	$(DBMATE) rollback

db-status: ## Show migration status
	$(DBMATE) status

db-new: ## Create new migration (usage: make db-new name=add_users)
	$(DBMATE) new $(name)
