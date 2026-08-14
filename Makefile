.PHONY: dev-backend dev-frontend build-backend build-frontend lint-frontend scan-secrets audit-frontend audit-backend test-backend

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
