# Asol kaj scripts/ e (Windows/Git Bash e make na thakle sorasori chalao).
.PHONY: quick-check check db-test rules db-up db-down migrate run web dev-seed
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
# Local catalog data (M3): docker er Postgres e, tai shudhu local; shop age (shop-api shop create)
dev-seed:
	docker compose -f infra/docker-compose.yml exec -T postgres psql -q -U shop_owner -d shop < scripts/dev-seed.sql
# backend/.env lage (cp backend/env.example backend/.env)
migrate:
	@bash scripts/dev.sh migrate
run:
	@bash scripts/dev.sh serve
# Website (Jaspr + Tailwind): website/.env lage (cp website/env.example website/.env)
web:
	@bash scripts/web.sh
