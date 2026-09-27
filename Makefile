.PHONY: dev dev-build dev-up dev-run dev-shell dev-down dev-clean test test-build test-up test-run test-down test-clean test-logs prod prod-build prod-up prod-down prod-clean prod-logs prod-save logs logs-nginx logs-app logs-save logs-save-nginx logs-save-app

COMPOSE_DEV := sudo docker compose -f docker-compose.yaml -f docker-compose.dev.yaml
COMPOSE_TEST := sudo docker compose -f docker-compose.yaml -f docker-compose.test.yaml
COMPOSE_PROD := sudo docker compose -f docker-compose.prod.yaml
COMPOSE := $(COMPOSE_DEV)

dev:
	$(COMPOSE) up -d --build

dev-build:
	$(COMPOSE_DEV) build

dev-up:
	$(COMPOSE_DEV) up -d

dev-run:
	$(COMPOSE) exec twivo-media go run .

dev-shell:
	$(COMPOSE) exec twivo-media bash

dev-down:
	$(COMPOSE) down

dev-clean:
	$(COMPOSE) down -v

test:
	$(COMPOSE_TEST) up -d --build

test-build:
	$(COMPOSE_TEST) build

test-up:
	$(COMPOSE_TEST) up -d

test-run: test
	$(COMPOSE_TEST) exec twivo-media bash

test-shell:
	$(COMPOSE_TEST) exec twivo-media bash

test-down:
	$(COMPOSE_TEST) down --remove-orphans

test-clean:
	$(COMPOSE_TEST) down -v --remove-orphans

test-logs:
	$(COMPOSE_TEST) logs -f --tail=100

prod:
	$(COMPOSE_PROD) up -d --build

prod-build:
	$(COMPOSE_PROD) build

prod-up:
	$(COMPOSE_PROD) up -d

prod-down:
	$(COMPOSE_PROD) down

prod-clean:
	$(COMPOSE_PROD) down -v

prod-logs:
	$(COMPOSE_PROD) logs -f --tail=100

prod-save: prod-down
	@set -e; mkdir -p backups/production; timestamp=$$(date +%Y%m%d-%H%M%S); for volume in twivo-media-prod-seaweed-master-data twivo-media-prod-seaweed-volume-data twivo-media-prod-seaweed-filer-data twivo-media-prod-mongo-data twivo-media-prod-redis-data; do sudo docker run --rm -v "$$volume:/data:ro" alpine:3.21 tar -czf - -C /data . > "backups/production/$$volume-$$timestamp.tar.gz"; done

logs:
	$(COMPOSE) logs -f --tail=100

logs-nginx:
	$(COMPOSE) logs -f --tail=100 nginx

logs-app:
	$(COMPOSE) logs -f --tail=100 twivo-media

logs-save:
	mkdir -p logs
	$(COMPOSE) logs --no-color > logs/compose-$$(date +%Y%m%d-%H%M%S).log

logs-save-nginx:
	mkdir -p logs
	$(COMPOSE) logs --no-color nginx > logs/nginx-$$(date +%Y%m%d-%H%M%S).log

logs-save-app:
	mkdir -p logs
	$(COMPOSE) logs --no-color twivo-media > logs/app-$$(date +%Y%m%d-%H%M%S).log
