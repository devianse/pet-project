---
status: accepted
---

# httpOnly, SameSite=Strict cookie for the session JWT

The frontend needs to hold a session credential across requests. We chose
a signed JWT stored in an httpOnly, `SameSite=Strict` cookie over the more
common SPA pattern of a token in `localStorage` read by JS and attached
as an `Authorization` header. The cookie is invisible to JS (no
XSS-exfiltration path) and `SameSite=Strict` blocks it being sent
cross-site (no CSRF token needed), at the cost of the frontend never
being able to inspect or manage the token itself — every auth-aware UI
decision goes through a `/api/me`-style call instead of decoding a
client-held token. Chosen for the security property, not for
convenience; a future cross-origin frontend (a separate domain, a mobile
app) would need a different mechanism, since `SameSite=Strict` cookies
don't survive that split.
