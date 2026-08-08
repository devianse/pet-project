# Notes — design

## Status

Deliberate detour ahead of `PLANNING.md`'s shell-first build order. Phase 2
(auth, WebSockets) has not started; this feature ships without auth, on
Postgres (pulled forward from its planned phase-3 role) rather than
waiting. Scope is intentionally small — a practice feature, not a
production-critical one.

## Scope

A standalone page at `/notes`: add plain-text items, remove them, list
persists across reloads/sessions via Postgres. No edit-in-place. No
per-item metadata beyond content and creation time. No auth — anyone who
can reach the app can read/add/remove notes, matching the rest of the
current phase-1 shell.

This is a new feature (`features/notes/`), not a fill-in for the existing
`Shopping List` placeholder — Shopping List is reserved for its real
phase-3 scope (meal-plan aggregation, per `PLANNING.md`).

## Backend approach

`database/sql` + `pgx` stdlib driver, raw SQL — no ORM, no query builder.
Matches the existing codebase's style (`net/http` stdlib mux, `godotenv`
for config, no framework). Considered and rejected:

- **pgx native pool** (non-`database/sql`): slightly more idiomatic to
  pgx specifically, but diverges from the portable `database/sql`
  interface for no real benefit at one-table scale.
- **ORM** (e.g. `gorm`): overkill for one table; also contradicts the
  raw-SQL-with-driver approach this feature is deliberately practicing.

## Data model

One table, created via `CREATE TABLE IF NOT EXISTS` at server startup —
no migration tool; too small a schema to justify one yet.

```sql
CREATE TABLE notes (
  id         SERIAL PRIMARY KEY,
  content    TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## API

- `GET /api/notes` → list, newest-first (`ORDER BY created_at DESC`),
  full array, no pagination
- `POST /api/notes` `{items: string[]}` → batch insert. Validates every
  string in `items` (see Validation below) — if any fails, the whole
  request is rejected (`400`) and nothing is inserted. On success,
  inserts all, returns the **full** notes list (not just the newly
  created rows), newest-first, same shape as `GET`
- `DELETE /api/notes/{id}` → removes exactly one note by id; `204` on
  success, `404` if no row matched. Deletes are single-item, not batched
  — this stays symmetric with the frontend's one-remove-button-per-item
  interaction.

## Validation

Enforced in the `POST /api/notes` handler, per item in the `items` array:

- empty or whitespace-only → `400`
- over 10,000 characters → `400`
- Malformed JSON body, or `items` missing/empty/not an array → `400`

Validation is all-or-nothing across the batch: one bad item fails the
whole request rather than silently dropping it, so the frontend never has
to reconcile "3 of 4 saved."

No request-body-size guard (e.g. `http.MaxBytesReader`) — considered and
explicitly deferred; the per-item char-count check is the only limit for
now.

## Frontend

New `features/notes/Page.tsx`:

- A text input + "Add" button that appends to a **local, unsaved staging
  list** (component state only — nothing hits the network yet)
- The staged items render below the input, each removable before saving
  (local-only removal, no API call)
- A "Save" button below the staged list — on click, batch-POSTs all
  staged items in one request, then replaces the page's item state
  wholesale with the response array (no merge logic needed, since the
  response is already the full authoritative list) and clears the
  staging list
- Previously-saved items (loaded via `GET` on page mount) each get their
  own "Remove" button, wired to the single-item `DELETE`

`shared/api.ts` gains `getNotes`, `createNotes` (batch), `deleteNote`,
following the existing `getHealth` pattern (plain `fetch`, no client
library).

## Local Postgres

Ad hoc `docker run postgres` for local dev — no `infra/` folder or
compose file yet, that stays deployment-time scope per `PLANNING.md`.
Connection string via a new `DATABASE_URL` var in `backend/.env.example`.

## Error handling

- Validation failures (see above) → `400` with a clear message
- DB errors (connection, query failure) → `500`, logged via the existing
  `slog` logger
- `DELETE` on a non-existent id → `404`

## Testing

Handler tests via `httptest`, run against a real local Postgres instance
(not mocked) — small enough scope that a real DB in tests is simpler than
mock plumbing.

## Open questions / explicitly deferred

- Request-body-size cap (`http.MaxBytesReader`) — flagged during design,
  deliberately left out for now
- Migration tooling — deferred until schema changes enough to need one
- Auth/ownership of notes — phase 2 concern, not this feature's problem
