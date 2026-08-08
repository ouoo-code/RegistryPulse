.PHONY: help dev build build-frontend build-backend test test-backend test-frontend test-e2e lint format frontend-format validate compose-smoke docker-build docker-up docker-down logs backup restore migrate seed clean
help:
	@echo "make dev build test test-backend test-frontend test-e2e lint format validate compose-smoke docker-build docker-up docker-down logs backup restore migrate seed clean"
build:
	$(MAKE) build-frontend
	$(MAKE) build-backend
build-frontend:
	cd frontend && npm install --no-audit --no-fund && npm run build
build-backend:
	cd backend && go build ./...
dev:
	docker compose up -d --build
test: test-backend test-frontend
test-backend:
	cd backend && go test ./...
test-frontend:
	cd frontend && npm install --no-audit --no-fund && npm test
test-e2e:
	sh tests/e2e-run.sh
lint:
	cd frontend && npm install --no-audit --no-fund && npx tsc --noEmit
	cd backend && go vet ./...
validate:
	sh tests/validate.sh
compose-smoke:
	sh tests/compose-smoke.sh
format:
	find backend -type f -name '*.go' -print0 | xargs -0 gofmt -w
frontend-format:
	cd frontend && npm run format --if-present
docker-build:
	docker compose build
docker-up:
	docker compose up -d
docker-down:
	docker compose down
logs:
	docker compose logs -f
backup:
	sh deploy/scripts/backup.sh
restore:
	@test -n "$(DIR)" || (echo "usage: make restore DIR=backups/YYYY-MM-DD_HH-mm-ss" && exit 2)
	sh deploy/scripts/restore.sh "$(DIR)"
migrate:
	sh deploy/scripts/migrate.sh
seed:
	ADMIN_API_TOKEN="$(ADMIN_API_TOKEN)" BASE_URL="$(BASE_URL)" sh deploy/scripts/seed.sh
clean:
	docker compose down --remove-orphans
