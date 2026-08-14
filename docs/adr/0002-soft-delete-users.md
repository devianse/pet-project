---
status: accepted
---

# Soft-delete users via `is_active`, never hard delete

Deactivating a User sets `is_active = false` and blocks login; the row
and every foreign-keyed record (audit log entries, Watchlist items, Date
Night proposals, ...) stays intact. Hard delete was considered and
rejected up front as bad practice for DB management here — it would
force a cascade-delete decision for every table a User can be referenced
from, and there's no real storage or compliance pressure at this scale
to justify it. Deactivation is fully reversible (reactivate flips the
flag back) and a deactivated login fails through the same generic 401 as
a wrong password, so there's no user-enumeration signal either. Revisit
only if a real requirement to actually erase a User's data shows up
(e.g. a privacy/compliance obligation) — that's a different feature, not
a toggle on this one.
