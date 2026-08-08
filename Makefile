.PHONY: dev-backend dev-frontend build-backend build-frontend lint-frontend

dev-backend:
	cd backend && go run ./cmd/api

dev-frontend:
	cd frontend && npm run dev

build-backend:
	cd backend && go build ./...

build-frontend:
	cd frontend && npm run build

lint-frontend:
	cd frontend && npm run lint
