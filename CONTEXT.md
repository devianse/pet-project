# Pet Projects

A personal portfolio platform: one shell (auth, per-user feature gating,
deployment) that self-contained domain projects (Notes, Watchlist, Date
Night, Telegram, and the still-unbuilt Shopping List / Image Processing)
plug into as pages/routes. Single bounded context — one Postgres
database, one `User` concept, no independent subdomains yet. See
`PLANNING.md` for architecture and `planning/history.md` for build order.

## Language

### Platform / access

**User**:
An account in the platform. Invite-only — no public registration; seeded
via `cmd/createuser`. Has a Role, an Active/Deactivated status, and zero
or more Grants.
_Avoid_: Account, Customer.

**Role**:
`admin` or `user`, one per User. `admin` bypasses Feature gating
automatically — re-checked against the database on every request, never
trusted from a (possibly stale) JWT claim.
_Avoid_: Permission level.

**Feature**:
One gate-able page/capability — the fixed set in `KnownFeatures`
(`notes`, `watchlist`, `date-night`, `shopping-list`, `image-processing`).
Not DB-editable; adding one is a code change, not admin-UI data entry.
_Avoid_: Module, Page.

**Grant**:
A User's access to one Feature (a `feature_access` row). Non-admin Users
start with zero Grants. `admin` bypasses Grants entirely rather than
being auto-granted every Feature.
_Avoid_: Permission, Access.

**Session**:
The logged-in state carried by a signed JWT in an httpOnly,
`SameSite=Strict` cookie.
_Avoid_: Token (the JWT itself, not the state it represents).

**Deactivated**:
A User with `is_active = false`. Soft-delete: blocks login, fully
reversible, indistinguishable from a wrong password at login (no
enumeration signal). The deliberate alternative to hard-deleting a User
row.
_Avoid_: Deleted, Disabled.

**Audit Log Entry**:
One row of an admin mutation — grant, revoke, create, deactivate,
reactivate, reset-password, or role-change — recording the actor, an
optional target User, and when it happened.
_Avoid_: Event, Activity log.

### Notes

**Note**:
A flat text entry. No per-user ownership yet — one implicit shared owner
across every User with the `notes` Grant.

### Watchlist

**Watchlist Item**:
A movie or TV title, resolved from a pasted IMDb link via TMDb, tracked
with a viewed/unviewed flag.
_Avoid_: Movie (a Watchlist Item may be a TV title).

### Date Night

**Activity**:
A Date Night category/tag drawn from a fixed set. Referenced by
Proposals; protected from deletion while any Proposal still references
it (no cascade — a Proposal's Activity is what the Proposal *means*).

**Proposal**:
A suggested day, time slot, and Activity, made by one of the two paired
Date Night accounts to the other. Has a Status (pending / accepted /
declined).

**Current Proposal**:
The most recently made Proposal for the pair. Only the Current Proposal
can be accepted or declined — an older pending Proposal can't be acted
on once a newer one exists.

### Telegram

**Telegram Command**:
A `/`-prefixed instruction (e.g. `/notes`, `/newnote`) routed by prefix
match to a handler. v1 is commands-in only, restricted to a single
allowed chat ID.
