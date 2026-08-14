.PHONY: dev-backend dev-frontend build-backend build-frontend lint-frontend scan-secrets audit-frontend audit-backend test-backend redeploy

dev-backend:
	cd backend && go run ./cmd/api

dev-frontend:
	cd frontend && npm run dev

build-backend:
	cd backend && go build ./...

# -p 1 runs one package's tests at a time. Several packages share
# unscoped tables (notes has no owner column yet — see
# planning/decisions.md's "notes/feature multi-tenancy" entry) so
# concurrent packages can race on the same rows under Go's default
# parallel-package test execution. Needs a live DATABASE_URL to run
# the integration tests; falls back to skipping them otherwise.
test-backend:
	cd backend && go test -p 1 ./...

build-frontend:
	cd frontend && npm run build

lint-frontend:
	cd frontend && npm run lint

scan-secrets:
	@command -v gitleaks >/dev/null 2>&1 || { echo "gitleaks not found — see README's 'Other commands' section for install instructions"; exit 1; }
	gitleaks detect --source . -v

audit-frontend:
	cd frontend && npm audit

audit-backend:
	@command -v govulncheck >/dev/null 2>&1 || { echo "govulncheck not found — run: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	cd backend && govulncheck ./...

# Computes GIT_SHA fresh in the same command that uses it, so it can
# never linger stale from an export left over in an earlier shell
# session (see planning/decisions.md's "Redeploy's GIT_SHA export is
# manual" entry). Run from the VPS after `git pull`, same as before.
redeploy:
	cd infra && GIT_SHA=$$(git -C .. rev-parse --short HEAD) docker compose up -d --build
