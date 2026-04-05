SHELL := /usr/bin/bash

# Variables
DOCKER_COMPOSE ?= docker compose
BUN ?= bun
# Default desk TV (TCP adb). Override: make deploy-tv PLUM_TV_ADB=192.168.1.5:5555
PLUM_TV_ADB ?= 192.168.2.11:5555
# Living-room TV (TCP adb). Override: make deploy-tv-lr PLUM_TV_ADB_LR=192.168.2.x:5555
PLUM_TV_ADB_LR ?= 192.168.2.20:5555

# Colors
BLUE := \033[1;34m
GREEN := \033[1;32m
YELLOW := \033[1;33m
RED := \033[1;31m
NC := \033[0m # No Color

.PHONY: help dev dev-clean dev-stop docker-dev docker-dev-clean build up down logs logs-app logs-frontend ps restart clean lint fmt test android-tv-build deploy-tv deploy-tv-reinstall deploy-tv-install deploy-tv-reinstall-install deploy-tv-lr deploy-tv-lr-reinstall deploy-tv-lr-install deploy-tv-lr-reinstall-install

# Default target
help:
	@echo "$(BLUE)╔════════════════════════════════════════════════════════════════╗$(NC)"
	@echo "$(BLUE)║             Plum - Full Stack Development CLI                ║$(NC)"
	@echo "$(BLUE)╚════════════════════════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(GREEN)Development:$(NC)"
	@echo "  make dev         - 🚀 Start local web + server dev and auto-open the mordor media-stack tunnel"
	@echo "  make dev-clean   - 🧹 Reset local dev state, then start dev with live console output"
	@echo "  make dev-stop    - ⛔ Stop the tracked mordor media-stack tunnel"
	@echo "  make docker-dev  - 🐳 Start the Docker dev stack"
	@echo "  make docker-dev-clean - 🧼 Recreate the Docker dev stack from scratch"
	@echo "  make build      - 🔨 Build all Docker images"
	@echo "  make up          - ⬆️  Start services in background"
	@echo "  make down        - ⬇️  Stop all services"
	@echo "  make restart     - 🔄 Restart all services"
	@echo ""
	@echo "$(GREEN)Logs:$(NC)"
	@echo "  make logs        - 📋 Stream all logs"
	@echo "  make logs-app    - 📋 Backend logs only"
	@echo "  make logs-frontend - 📋 Frontend logs only"
	@echo ""
	@echo "$(GREEN)Code Quality:$(NC)"
	@echo "  make lint        - 🔍 Lint both backend and frontend"
	@echo "  make fmt         - 🎨 Format both backend and frontend"
	@echo ""
	@echo "$(GREEN)Testing:$(NC)"
	@echo "  make test        - 🧪 Run backend tests"
	@echo ""
	@echo "$(GREEN)Android TV:$(NC)"
	@echo "  make android-tv-build - 🔨 Build the Android TV debug APK"
	@echo "  make deploy-tv   - 📺 Build release APK, adb connect desk TV, install, launch (PLUM_TV_ADB)"
	@echo "  make deploy-tv-install - 📺 Same as deploy-tv but do not launch the app"
	@echo "  make deploy-tv-reinstall - 📺 Same, after adb uninstall (signature mismatch / fresh install)"
	@echo "  make deploy-tv-reinstall-install - 📺 Reinstall desk TV, no launch"
	@echo "  make deploy-tv-lr - 📺 Build release APK, adb connect LR TV, install, launch (PLUM_TV_ADB_LR)"
	@echo "  make deploy-tv-lr-install - 📺 LR TV install only, no launch"
	@echo "  make deploy-tv-lr-reinstall - 📺 Same as deploy-tv-lr with uninstall first"
	@echo "  make deploy-tv-lr-reinstall-install - 📺 LR reinstall, no launch"
	@echo ""
	@echo "$(GREEN)Cleanup:$(NC)"
	@echo "  make clean       - 🧹 Remove containers, volumes, and temp files"

dev:
	./scripts/dev.sh

dev-clean:
	./scripts/dev-clean.sh

dev-stop:
	./scripts/dev-stop.sh

docker-dev:
	# Start full stack. Rebuilds only what's changed.
	$(DOCKER_COMPOSE) up --build

docker-dev-clean:
	# Remove volumes for a fresh DB (onboarding from scratch) and clean caches.
	$(DOCKER_COMPOSE) down -v
	$(DOCKER_COMPOSE) up --build

build:
	# Explicit rebuild of all images without starting containers.
	$(DOCKER_COMPOSE) build

up:
	# Start in background, rebuilding images only if needed.
	$(DOCKER_COMPOSE) up -d --build

down:
	$(DOCKER_COMPOSE) down

restart:
	$(DOCKER_COMPOSE) restart

logs:
	$(DOCKER_COMPOSE) logs -f

logs-app:
	$(DOCKER_COMPOSE) logs -f app

logs-frontend:
	$(DOCKER_COMPOSE) logs -f frontend

ps:
	$(DOCKER_COMPOSE) ps

lint: lint-backend lint-frontend

lint-backend:
	cd apps/server && golangci-lint run

lint-frontend:
	cd apps/web && $(BUN) run lint

fmt: fmt-backend fmt-frontend

fmt-backend:
	cd apps/server && ../../scripts/go.sh fmt ./...

fmt-frontend:
	cd apps/web && $(BUN) run format

test:
	cd apps/server && ../../scripts/go.sh test -v ./...

android-tv-build:
	./scripts/android-tv.sh :app:assembleDebug

deploy-tv:
	env PLUM_TV_ADB="$(PLUM_TV_ADB)" bash ./scripts/android-tv-deploy-desk.sh

deploy-tv-reinstall:
	env PLUM_TV_ADB="$(PLUM_TV_ADB)" PLUM_TV_REINSTALL=1 bash ./scripts/android-tv-deploy-desk.sh

deploy-tv-install:
	env PLUM_TV_ADB="$(PLUM_TV_ADB)" PLUM_TV_NO_LAUNCH=1 bash ./scripts/android-tv-deploy-desk.sh

deploy-tv-reinstall-install:
	env PLUM_TV_ADB="$(PLUM_TV_ADB)" PLUM_TV_REINSTALL=1 PLUM_TV_NO_LAUNCH=1 bash ./scripts/android-tv-deploy-desk.sh

deploy-tv-lr:
	env PLUM_TV_ADB_LR="$(PLUM_TV_ADB_LR)" bash ./scripts/android-tv-deploy-lr.sh

deploy-tv-lr-reinstall:
	env PLUM_TV_ADB_LR="$(PLUM_TV_ADB_LR)" PLUM_TV_REINSTALL=1 bash ./scripts/android-tv-deploy-lr.sh

deploy-tv-lr-install:
	env PLUM_TV_ADB_LR="$(PLUM_TV_ADB_LR)" PLUM_TV_NO_LAUNCH=1 bash ./scripts/android-tv-deploy-lr.sh

deploy-tv-lr-reinstall-install:
	env PLUM_TV_ADB_LR="$(PLUM_TV_ADB_LR)" PLUM_TV_REINSTALL=1 PLUM_TV_NO_LAUNCH=1 bash ./scripts/android-tv-deploy-lr.sh

clean:
	$(DOCKER_COMPOSE) down -v
	rm -rf apps/server/tmp apps/server/bin apps/web/dist
