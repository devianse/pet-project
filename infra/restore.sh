#!/usr/bin/env bash
# Restores a pg_dump custom-format backup into a running Postgres
# container. Primary use: after provisioning a fresh VPS during a
# migration, run this against infra/docker-compose.yml's postgres
# service before cutting DNS over — see
# infra/deployment-runbook/04-restore-backup.md. Also used to test the
# backup/restore round-trip locally (see infra/restore_test.sh).
#
# Usage: infra/restore.sh <dump-file> [container]
#   <dump-file>  path to a .dump file produced by infra/backup.sh (or
#                any pg_dump -Fc output)
#   [container]  container name/ID to restore into. Defaults to the
#                "postgres" service of the docker-compose.yml in the
#                current directory (run this from infra/, same as
#                every other docker compose command in this repo).
#                Pass this explicitly to target a standalone container
#                instead (e.g. for local testing).
set -euo pipefail

usage() {
  echo "Usage: infra/restore.sh <dump-file> [container]" >&2
}

main() {
  if [[ $# -lt 1 || $# -gt 2 ]]; then
    usage
    exit 1
  fi
  local dump_file="$1"
  local container="${2:-}"

  if [[ ! -f "$dump_file" ]]; then
    echo "restore.sh: no such file: $dump_file" >&2
    exit 1
  fi
  if [[ ! -s "$dump_file" ]]; then
    echo "restore.sh: dump file is empty: $dump_file" >&2
    exit 1
  fi

  if [[ -z "$container" ]]; then
    container=$(docker compose ps -q postgres)
    if [[ -z "$container" ]]; then
      echo "restore.sh: no running 'postgres' service found via docker compose (run from infra/, or pass a container name explicitly)" >&2
      exit 1
    fi
  fi

  echo "restoring $dump_file into container $container ..."
  docker exec -i "$container" sh -c 'pg_restore --clean --if-exists -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$dump_file"
  echo "restore complete"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
