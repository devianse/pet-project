---
status: accepted
---

# One VPS, no Kubernetes or load balancer

The whole stack (Caddy + Go API + Postgres) runs as three containers via
one `docker-compose.yml` on a single VPS. No orchestrator, no load
balancer, no multi-instance deployment. This is a deliberate scope
decision, not a gap: LB/orchestration solve a scaling problem this
platform doesn't have at personal-portfolio traffic levels, and they'd
add real operational surface (cluster config, service discovery,
multi-node secrets) for no present benefit. Revisit only if actual
traffic or availability requirements outgrow one instance — not
preemptively.
