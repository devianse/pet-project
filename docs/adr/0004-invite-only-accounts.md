---
status: accepted
---

# Invite-only accounts, no public registration endpoint

There is no signup flow. Every account is seeded by an operator via
`cmd/createuser` (locally or `docker compose exec backend createuser`
against the deployed DB). This is a personal/family-scale platform, not
a public product — a registration endpoint would be pure attack surface
(spam accounts, credential-stuffing target, email-verification
infrastructure) with no corresponding benefit, since every real user is
known ahead of time. Revisit only if the platform ever needs to onboard
users the operator doesn't personally know.
