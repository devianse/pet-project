# Workflow — taking a change from finding to merged

Written so a fresh session (or a person) can run the whole loop without
re-deriving it. `CLAUDE.md` says *what* the conventions are; this is the
mechanical *how*, including the parts that silently fail if you skip
them.

## 0. Before anything: get a working test database

This is the step that bites. Most backend tests are integration tests
against a real Postgres and **`t.Skip` themselves when `DATABASE_URL` is
unset** — so `go test ./...` prints a wall of `ok` while having
asserted almost nothing. On a machine with no database the split is
roughly 103 real passes against 139 skips.

Check which situation you are in:

```sh
cd backend && go test -p 1 -v ./... 2>&1 | grep -cE '^\s*--- SKIP'
```

A non-zero count means the integration tier did not run.

Start the database (see `CHEATSHEET.local/02-postgres-db.md` for the
canonical container):

```sh
docker start pet-projects-notes-db
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/notes?sslmode=disable'
```

If the container exists but was created without `-p 5432:5432`, it
starts fine and is still unreachable from the host — the tests then skip
rather than fail, which looks like success. Recreate it with the port
mapping (the data lives in a named volume, so `-v
pet-projects-notes-db-data:/var/lib/postgresql/data` preserves it).

## 1. One scope, one worktree, one branch

Never edit on `main`. Each independent scope gets its own worktree so
they can be built and verified in parallel without stashing:

```sh
git fetch origin
git worktree add -b fix/short-scope ../pp-wt-scope origin/main
```

Split scopes so each PR could be reverted on its own. A dependency bump,
a docs refresh, and a behaviour fix are three PRs even if found in one
sitting — they have different review and revert stories.

## 2. Make the change, then run the real gates

From the worktree:

```sh
# backend
cd backend && gofmt -l . && go build ./... && go vet ./...
go test -p 1 ./...          # needs DATABASE_URL from step 0
go test -race -p 1 ./...    # nothing in CI runs this; do it by hand

# frontend
cd frontend && npx tsc -b && npm run build && npm run test && npm run lint
```

`-p 1` is required, not stylistic: several packages share unscoped
tables (notes has no owner column yet) and race on the same rows under
Go's default parallel-package execution. See the Makefile note.

## 3. The three scans that gate a PR

Per `CLAUDE.md`, a PR is not ready until all three are clean:

```sh
make audit-frontend   # npm audit
make audit-backend    # govulncheck ./...
make scan-secrets     # gitleaks detect --source . -v
```

Two things that trip people up:

- **`govulncheck` must be built with a Go version at least as new as
  `go.mod`'s `go` directive**, or it fails with `package requires newer
  Go version`. Reinstall it under the current toolchain after any Go
  bump: `go install golang.org/x/vuln/cmd/govulncheck@latest`.
- **A pre-existing failure on `main` is not your regression, but it does
  block the bar.** Say so in the PR rather than quietly ignoring it, and
  if the fix is independent, open it as its own PR first so later PRs
  inherit a clean base.

Stdlib findings in `govulncheck` are fixed by moving the Go patch
release, not by code changes — and that also means bumping
`backend/Dockerfile`'s `golang:` base image, or the deploy build and
the local build drift apart.

## 4. Commit and open the PR

Commit as you go; **do not push or open a PR unless asked** — staging is
the user's call every time.

```sh
git add -A
git commit -m "..."          # explain why, not what; the diff shows what
git push -u origin fix/short-scope
gh pr create --base main --head fix/short-scope \
  --title "..." --body "..."
```

Put in the body: what breaks without this, what was verified (with the
actual numbers — "242 pass, 0 skip" beats "tests pass"), and what was
**deliberately left out** and why. The last part is what stops the next
session from assuming an omission was an oversight.

## 5. Clean up the worktree

Once the branch is pushed, the worktree has served its purpose — the PR
holds the diff and the branch lives on `origin`.

```sh
git worktree remove ../pp-wt-scope --force
git worktree list            # confirm it is gone
```

The local branch ref stays behind and is harmless; prune it after merge
with `git fetch --prune` (or `git branch -d fix/short-scope`).

Do not leave stale worktrees around: they pin `origin/main` checkouts in
place, and a forgotten one is indistinguishable from in-progress work.

## 6. After merge

```sh
git checkout main && git pull
git fetch --prune
git worktree prune
```

Then update the planning docs to the shipped state — `planning/history.md`
(entry + real PR number), `PLANNING.md` if the "Still open" list moved,
and `CLAUDE.md` if the phase section is now wrong. Stale orientation
docs are worse than none: every later session reads them first and
inherits the error.

## What this repo has no enforcement for

There is no CI. Every gate above is manual and convention-enforced. That
is a known, accepted gap (`planning/security-todo.md`), not an
oversight — which is exactly why steps 2 and 3 are written out instead
of left to a pipeline.
