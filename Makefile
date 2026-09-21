.PHONY: dev dev-shell dev-down dev-clean logs logs-nginx logs-app logs-save logs-save-nginx logs-save-app

COMPOSE := sudo docker compose -f docker-compose.yaml -f docker-compose.dev.yaml

dev:
	$(COMPOSE) up -d --build

dev-shell:
	$(COMPOSE) exec twivo-media bash

dev-down:
	$(COMPOSE) down

dev-clean:
	$(COMPOSE) down -v

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
