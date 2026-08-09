# Movie/TV Sharing List — design

## Status

Deliberate detour ahead of `PLANNING.md`'s shell-first build order, same
pattern as Notes and pre-deployment security hardening (step 5 of the
"Actual build order so far" section). Phase 2 (auth, WebSockets) has not
started; this feature ships without auth, on Postgres, as a single shared
list — anyone who can reach the app can read/add/toggle/remove items, the
same no-auth model Notes already established. Comes before the first VPS
deployment attempt (step 6), not after.

Requires a TMDb API Read Access Token, obtained manually beforehand — see
"Prerequisites" below.

## Scope

A standalone page at `/watchlist`: paste an IMDb title link, the backend
resolves it against TMDb and adds it to the shared list as a card
(poster, title, year, movie/TV badge). Cards can be expanded for more
detail (overview, genres, rating), marked viewed/unwatched, or removed.
No per-user ownership, no separate lists, no edit-in-place beyond the
viewed toggle.

This is a new feature (`features/watchlist/`), independent of Notes and
of the still-unbuilt phase-3 Shopping List.

## Prerequisites (manual, one-time, before implementation)

- A TMDb account → Settings → API → request a "Developer" API key. The
  form's "Application URL" field is free text, not validated as a live,
  reachable endpoint — `http://localhost:3000` works and doesn't need to
  match the eventual Cloudzy domain. No dependency on deployment
  happening first, despite the form's wording suggesting otherwise.
- Specifically the **Read Access Token (v4 auth)**, not the shorter v3
  API key — used as an `Authorization: Bearer <token>` header, TMDb's
  current recommended method (v3 keys are the older `?api_key=`
  query-param style and still work, but aren't what this design uses).
- TMDb's [terms of use](https://www.themoviedb.org/api-terms-of-use)
  require attribution — a visible "This product uses the TMDB API but is
  not endorsed or certified by TMDB" notice somewhere on the page (e.g.
  the `/watchlist` footer).

## Backend approach

Same conventions as Notes: `database/sql` + `pgx` stdlib driver, raw
SQL, no ORM. TMDb calls use the stdlib `net/http` client directly — no
third-party TMDb SDK dependency, matching the existing no-framework,
minimal-dependency style.

## Data model

One table, created via `CREATE TABLE IF NOT EXISTS` at server startup —
no migration tool, same as Notes.

```sql
CREATE TABLE watchlist_items (
  id           SERIAL PRIMARY KEY,
  imdb_id      TEXT NOT NULL UNIQUE,       -- e.g. "tt0458290"
  media_type   TEXT NOT NULL,              -- 'movie' | 'tv'
  tmdb_id      INTEGER NOT NULL,
  title        TEXT NOT NULL,
  release_year TEXT,                       -- nullable, TMDb sometimes lacks it
  poster_path  TEXT,                       -- nullable; NULL = placeholder in UI
  overview     TEXT,
  vote_average REAL,
  genres       TEXT,                       -- comma-joined genre names, denormalized
  viewed       BOOLEAN NOT NULL DEFAULT false,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`imdb_id UNIQUE` is the source of truth for duplicate rejection — the
`POST` handler checks it proactively so the error is a friendly message,
not a raw constraint violation surfaced to the user, with the DB
constraint itself as the backstop against a race between two concurrent
adds of the same link.

## TMDb integration

- `TMDB_READ_ACCESS_TOKEN` in `backend/.env` / `.env.example`, loaded via
  `godotenv`, same convention as `PORT`/`DATABASE_URL`. Only a
  placeholder value goes in `.env.example` — the real token stays in the
  gitignored `.env`, never copy-pasted into the checked-in file.
- One outbound call per add: `GET
  https://api.themoviedb.org/3/find/{imdb_id}?external_source=imdb_id`,
  with an `Authorization: Bearer {token}` header. The response has
  `movie_results[]` and `tv_results[]`; whichever is non-empty sets
  `media_type` and determines which fields to read (`title`/
  `release_date` for movies vs. `name`/`first_air_date` for TV),
  normalized into the row shape above. Both arrays empty → `400`, "no
  title found for that link," nothing inserted. Because the lookup is by
  exact external id rather than fuzzy title search, there's no
  multiple-candidates ambiguity to resolve — `/find` either returns
  exactly one match per media type or none.
- Genre names: `/find` only returns numeric `genre_ids`, not names.
  Fetch `/genre/movie/list` and `/genre/tv/list` **once at backend
  startup**, cache the id→name maps in memory (~19 + ~16 entries,
  effectively static data), and use them to resolve `genre_ids` into the
  `genres` string stored at insert time. No per-request lookup, no
  separate genre table.
- Poster URLs are hot-linked directly from `image.tmdb.org` by the
  frontend (`https://image.tmdb.org/t/p/w342/{poster_path}`, or a
  placeholder if `poster_path` is `NULL`) — no backend image proxy.
  Considered and rejected: proxying images through the backend to avoid
  any dependency on TMDb's CDN being reachable from a viewer's network.
  Rejected because viewers are a fixed, VPN-always population by
  requirement — if TMDb's Russia/Belarus IP block (confirmed current
  behavior, not merely suspected) affects a non-VPN viewer, that's
  explicitly out of scope to solve for. `infra/Caddyfile`'s CSP allows
  `https://image.tmdb.org` in `img-src` specifically for this — removing
  it silently breaks every poster image.
- Deployment note (not a code concern, recorded so it isn't
  re-litigated): TMDb blocks by request source IP, not by domain
  registrar or account origin. The backend's outbound calls will
  originate from the Cloudzy VPS once deployed, which is not hosted in a
  Russia/Belarus region — so server-side TMDb calls are unaffected by
  that block regardless of where the domain is registered or who's
  browsing.

## API

- `GET /api/watchlist` → full list, `ORDER BY created_at DESC`, no
  pagination.
- `POST /api/watchlist` `{link: string}` → parses an IMDb title URL
  (`imdb.com/title/tt.../...`), extracts the `tt` id, calls TMDb,
  inserts, returns the new row. `400` on: malformed/non-IMDb URL, no
  TMDb match for the id, or duplicate `imdb_id` (friendly "already on
  the list" message rather than a raw DB error).
- `PATCH /api/watchlist/{id}` `{viewed: bool}` → toggles the flag; `404`
  if no row matches.
- `DELETE /api/watchlist/{id}` → removes exactly one item by id; `204`
  on success, `404` if no row matched. Same shape as Notes' delete
  endpoint.

## Validation

Enforced in the `POST /api/watchlist` handler:

- Link must match an IMDb title URL pattern (`imdb.com/title/tt<digits>`
  with optional path/query suffix) → otherwise `400`.
- Extracted `tt<digits>` id must resolve via TMDb's `/find` to a non-empty
  `movie_results` or `tv_results` → otherwise `400`.
- `imdb_id` must not already exist in `watchlist_items` → otherwise
  `400` with a friendly "already on the list" message.
- Malformed JSON body, or `link` missing/empty/not a string → `400`.

Unlike Notes' batch insert, `POST /api/watchlist` handles one link per
request — there's no batching need here since each add requires its own
TMDb round-trip and its own success/failure outcome.

## Frontend

- New `features/watchlist/Page.tsx`: a text input + submit button for
  pasting a link, with the list of cards rendered below it, and the TMDb
  attribution notice in the page footer.
- Card (collapsed): poster thumbnail (or a placeholder graphic when
  `poster_path` is `NULL`), title, year, a movie/TV badge, a viewed
  checkbox, and a remove button.
- Expand (click on the card): reveals the overview text, genres, and
  TMDb vote average. Purely a frontend show/hide of already-fetched
  data — no additional network request on expand.
- Viewed toggle is a simple boolean per item; toggling doesn't reorder
  or filter the list. No "watched" section split, no filter control —
  matching Notes' minimal-first pattern; can be revisited if the list
  grows long enough to need it.
- A failed `POST` (any of the validation cases above) renders its error
  inline near the input, mirroring how Notes surfaces validation errors
  — no toast/global banner.

## Testing

Matches Notes' scale: backend handler tests for the parse/validate/
insert/duplicate paths, with TMDb calls mocked (an injectable HTTP
client or interface, not live network calls in tests). No dedicated
frontend test suite, consistent with the rest of the app so far.

## Explicitly out of scope

- **Kinopoisk link support** — IMDb links only for v1; TMDb has no
  direct external-id lookup for Kinopoisk ids, so supporting it would
  mean scraping the Kinopoisk page and fuzzy-matching against TMDb's
  title search, a meaningfully different (and less reliable) code path.
  Flagged as a real future extension, not forgotten.
- **Folding the existing URL shortener into this feature** — raised
  during discussion as a "what if the pasted link went through the
  shortener too" idea. Independent feature, not needed for v1.
- **Backend image proxying** — see TMDb integration section above for
  why this was considered and rejected for now.
- **Cast/runtime detail** — would require a second TMDb call per item
  (`/movie/{id}` or `/tv/{id}`, since `/find` doesn't include them) at
  add-time; not needed for the expand view as scoped.
