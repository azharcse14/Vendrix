# Asol kaj scripts/ e (Windows/Git Bash e make na thakle sorasori chalao).
.PHONY: quick-check check db-test rules db-up db-down migrate run web
quick-check:
	@bash scripts/quick-check.sh
check:
	@bash scripts/check.sh
db-test:
	@bash scripts/db-test.sh
rules:
	@bash scripts/check-rules.sh
# Local Postgres 16 (C.UTF-8, shop_owner + shop_app): infra/docker-compose.yml
db-up:
	docker compose -f infra/docker-compose.yml up -d --wait
db-down:
	docker compose -f infra/docker-compose.yml down
# backend/.env lage (cp backend/env.example backend/.env)
migrate:
	@bash scripts/dev.sh migrate
run:
	@bash scripts/dev.sh serve
# Website (Jaspr + Tailwind): website/.env lage (cp website/env.example website/.env)
web:
	@bash scripts/web.sh
