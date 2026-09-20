.PHONY: dev dev-shell dev-down

COMPOSE := sudo docker compose -f docker-compose.yaml -f docker-compose.dev.yaml

dev:
	$(COMPOSE) up -d --build

dev-shell:
	$(COMPOSE) exec twivo-media bash

dev-down:
	$(COMPOSE) down
