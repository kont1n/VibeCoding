SHELL := /bin/bash

MODELS_DIR ?= models
MODELS_ARCHIVE ?= $(MODELS_DIR)/buffalo_l.zip
MODELS_URL ?= https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_l.zip

DOCKER_COMPOSE_FILE ?= deploy/compose/docker-compose.yml
DB_PASSWORD ?= facegrouper
REDIS_PASSWORD ?= facegrouper

.PHONY: help models-check models-download models-ensure docker-cpu-up docker-cpu-down docker-cpu-logs

help:
	@echo "Доступные цели:"
	@echo "  make models-check      - проверить наличие ONNX моделей"
	@echo "  make models-download   - скачать и распаковать модели"
	@echo "  make models-ensure     - проверить/скачать модели при необходимости"
	@echo "  make docker-cpu-up     - запустить Docker CPU стек (с автопроверкой моделей)"
	@echo "  make docker-cpu-down   - остановить Docker CPU стек"
	@echo "  make docker-cpu-logs   - смотреть логи CPU-сервиса"

models-check:
	@test -f "$(MODELS_DIR)/det_10g.onnx" || (echo "Отсутствует $(MODELS_DIR)/det_10g.onnx"; exit 1)
	@test -f "$(MODELS_DIR)/w600k_r50.onnx" || (echo "Отсутствует $(MODELS_DIR)/w600k_r50.onnx"; exit 1)
	@echo "OK: модели найдены в $(MODELS_DIR)"

models-download:
	@mkdir -p "$(MODELS_DIR)"
	@echo "Скачиваю модели в $(MODELS_DIR)..."
	@curl -fL "$(MODELS_URL)" -o "$(MODELS_ARCHIVE)"
	@unzip -o "$(MODELS_ARCHIVE)" -d "$(MODELS_DIR)"
	@rm -f "$(MODELS_ARCHIVE)"
	@$(MAKE) models-check

models-ensure:
	@if [ -f "$(MODELS_DIR)/det_10g.onnx" ] && [ -f "$(MODELS_DIR)/w600k_r50.onnx" ]; then \
		echo "OK: модели уже есть в $(MODELS_DIR)"; \
	else \
		$(MAKE) models-download; \
	fi

docker-cpu-up: models-ensure
	@DB_PASSWORD="$(DB_PASSWORD)" REDIS_PASSWORD="$(REDIS_PASSWORD)" \
	docker compose -f "$(DOCKER_COMPOSE_FILE)" up -d postgres redis face-grouper-cpu
	@echo "CPU стек запущен: http://localhost:8080"

docker-cpu-down:
	@DB_PASSWORD="$(DB_PASSWORD)" REDIS_PASSWORD="$(REDIS_PASSWORD)" \
	docker compose -f "$(DOCKER_COMPOSE_FILE)" down

docker-cpu-logs:
	@DB_PASSWORD="$(DB_PASSWORD)" REDIS_PASSWORD="$(REDIS_PASSWORD)" \
	docker compose -f "$(DOCKER_COMPOSE_FILE)" logs -f face-grouper-cpu
