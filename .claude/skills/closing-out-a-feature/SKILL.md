---
name: closing-out-a-feature
description: Use when implementation on a feature branch in this repo is complete and tests pass, before deciding how to integrate — verifies security/tests, checks for env/dependency/schema drift, pushes the branch and opens the PR, updates planning docs in their final already-shipped state citing the real PR number, and kills any dev servers or ad hoc docker resources this session started. Runs before finishing-a-development-branch, which owns the squash-merge/keep decision — by the time it runs, the PR already exists.
---

# Closing Out a Feature

## Overview

This repo's definition of done is more than a green test suite: planning
docs (`planning/history.md`, `PLANNING.md`, `infra/deployment-runbook/`,
`CHEATSHEET.local/`, `CONTEXT.md`, `docs/adr/`) need to stay in sync with
what shipped, and
subagent-driven development tends to leave dev servers and ad hoc docker
containers running. This skill closes both gaps before handing off to
`finishing-a-development-branch` for the squash-merge/keep decision — it
does not replace that skill, it runs immediately before it, and by then
the branch is already pushed with an open PR (Step 3) so the docs in
Step 4 can cite something real instead of a placeholder.

**Announce at start:** "I'm using the closing-out-a-feature skill before
we decide how to integrate this."

## Step 1: Verify

Run the project's test suites (backend `go test ./...`, frontend if it
has tests), then:

```bash
make scan-secrets audit-frontend audit-backend
```

**If anything fails**, report it and stop — don't proceed to later steps
on a red build. This mirrors `finishing-a-development-branch`'s own
test gate; running it here means the security/audit checks CLAUDE.md
requires are no longer something to remember to do "by hand."

## Step 2: Drift Checks

Diff the feature branch against its base branch and check for:

- **Env vars**: any new/changed var referenced in backend or frontend
  code, or in `infra/docker-compose.yml`, actually present in the
  matching `.env.example` — not just in a local gitignored `.env`.
- **New dependencies**: `go.mod` or `frontend/package.json` gained an
  entry. Not a blocker — just surface it explicitly (the existing
  history.md entries already call this out by hand, e.g. "no new
  dependency was added"; make that automatic instead of remembered).
- **Schema/CLI surface**: the branch touched a DB table (new column,
  new table) or added/changed a `backend/cmd/*` CLI tool. If so, flag
  that `CHEATSHEET.local/02-postgres-db.md` (schema queries, CLI list)
  may need a matching update — decide relevance in Step 3, don't edit
  here.

Report findings plainly; nothing here blocks progress, it just feeds
Step 3.

## Step 3: Push and Open the PR

This repo's convention: push the branch, open a GitHub PR, and
squash-merge from the GitHub UI once ready — no local merge commit (see
`finishing-a-development-branch`'s Quick Reference: only its "push and
create PR" option fits this workflow; its "merge locally" option
produces a real merge commit and should not be used here). Do this now,
before drafting docs, so Step 4 can cite a real PR number instead of a
placeholder or "pending" language:

```bash
git push -u origin <branch>
gh pr create --fill   # or with an explicit title/body if --fill doesn't fit
```

Capture the PR number from the output — Step 4 needs it. If a PR
already exists for this branch (e.g. re-running this skill after review
feedback added another commit), skip creation, just push, and reuse the
existing number.

## Step 4: Update Planning Docs

For each doc below, decide relevance, draft the update, show the draft,
and only write it after approval — these are narrative/judgment calls,
not mechanical edits.

**Write every doc in its final, already-shipped state — never a
"pending"/"not yet merged" hedge.** This branch's docs commit rides
inside the PR and lands on `main` verbatim when it's squash-merged, so
by the time anyone reads it there it *will* be true. Treating "PR open,
ready to merge" as done — rather than waiting for the literal
git-merge event — means:

- A `decisions.md` item this branch resolves moves straight out of
  "Still open" into the strikethrough/resolved list at the top,
  referencing the new `history.md` entry, instead of being left
  half-migrated for a later session to finish.
- The new `history.md` entry is written in the same finished,
  past-tense style as every other entry, citing the real PR number
  from Step 3 (e.g. "merged to `main` via PR #19") — not a placeholder
  or "will merge via."

The one case this doesn't cover: a PR that doesn't land as opened
(abandoned, substantially reworked in review). That's rare enough on a
solo-reviewed project not to design the common path around — if it
happens, fix the affected docs by hand then, as a one-off.

- **`planning/history.md`** — always: append the next numbered entry,
  matching the existing style (what shipped, why, backend/frontend
  summary, branch/PR reference, notable deferrals or gaps). Use Step
  1/2's findings (new dependency, schema change) as material for the
  entry the way existing entries already do.
- **`PLANNING.md`** — only if phase status or architecture changed.
- **`infra/deployment-runbook/`** — only if a deployment-relevant file
  changed (docker-compose, Caddyfile, env surface touching prod).
- **`CHEATSHEET.local/`** — only if Step 2 flagged schema/CLI drift, or
  a new dev command/gotcha came up this session.
- **`CONTEXT.md`** — only if the branch introduced or resolved a domain
  term (a new concept, a renamed one, a term used two different ways).
  Use `domain-modeling`'s format and rules rather than freehand editing.
- **`docs/adr/`** — only if the branch made a decision that's hard to
  reverse, would surprise a future reader, and came from a real
  trade-off (all three — see `domain-modeling`'s ADR criteria). Most
  branches won't clear this bar; don't force one.

Skip a doc outright rather than drafting a no-op update — say which
ones you're skipping and why in one line each.

## Step 5: Clean Up Dev Resources

Scope is **this session only** — don't go hunting for things older
sessions may have left running; that's a separate concern.

1. Check for backend/frontend dev processes this session started
   (`go run ./cmd/api`, `vite`/`npm run dev`) and stop them.
2. Check for docker containers/volumes this session created — ad hoc
   test databases, throwaway containers. Stop and remove them.
3. **Never touch `pet-projects-notes-db`** (container or volume) —
   it's the persistent local dev database, not session scratch. It's
   the only long-lived resource in this repo's local dev setup today;
   if that changes, update this list rather than guessing at cleanup
   time.

If something is running and you're not sure whether this session
started it or it predates this work, ask rather than killing it.

## Step 6: Hand Off

Once Steps 1-5 are done, invoke `finishing-a-development-branch` for
the squash-merge/keep decision. The branch is already pushed with an
open PR from Step 3 — when that skill's menu comes up, its "push and
create PR" option is already satisfied; report the existing PR link
instead of pushing/creating again. Don't make the merge/keep call here.
