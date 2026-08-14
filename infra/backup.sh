#!/usr/bin/env bash
# Dumps the production Postgres DB over SSH and pulls it to this
# machine in one command (no dump file is ever left on the VPS).
# Prunes local backups down to the 5 most recent for the same db.
#
# Usage: infra/backup.sh <vps-host>
#   e.g. infra/backup.sh myuser@1.2.3.4
#
# BACKUP_DIR (env, optional): where dumps are written locally.
#   Defaults to ~/pet-projects-backups — deliberately outside the repo
#   clone so a dump full of usernames/password hashes/notes content
#   can never land inside a `git add -A`.
set -euo pipefail
umask 077

usage() {
  echo "Usage: infra/backup.sh <vps-host>" >&2
  echo "  e.g. infra/backup.sh myuser@1.2.3.4" >&2
}

# Deletes all but the $keep most recent "$dir/pet-projects-$db-*.dump"
# files. Filenames are timestamp-sortable by construction
# (pet-projects-<db>-YYYYMMDD-HHMMSS.dump), so a plain sort is enough.
prune_old_backups() {
  local dir="$1" db="$2" keep="${3:-5}"
  local files=()
  while IFS= read -r f; do
    files+=("$f")
  done < <(find "$dir" -maxdepth 1 -name "pet-projects-$db-*.dump" | sort)
  local total="${#files[@]}"
  (( total <= keep )) && return 0
  local to_delete=$(( total - keep ))
  local i
  for (( i = 0; i < to_delete; i++ )); do
    rm -f -- "${files[$i]}"
    echo "pruned old backup: ${files[$i]}"
  done
}

# Runs a shell command inside the VPS's running postgres container,
# over SSH, via the existing docker-compose service (same pattern the
# deploy runbook already uses for createuser/grantaccess).
remote_exec() {
  local vps_host="$1" remote_cmd="$2"
  ssh "$vps_host" "cd ~/pet-project/infra && docker compose exec -T postgres sh -c '$remote_cmd'"
}

main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 1
  fi
  local vps_host="$1"
  local backup_dir="${BACKUP_DIR:-$HOME/pet-projects-backups}"
  mkdir -p "$backup_dir"

  local db
  db=$(remote_exec "$vps_host" 'echo "$POSTGRES_DB"' | tr -d '\r') \
    || { echo "backup.sh: failed to read POSTGRES_DB from $vps_host" >&2; exit 1; }
  if [[ -z "$db" ]]; then
    echo "backup.sh: POSTGRES_DB came back empty from $vps_host" >&2
    exit 1
  fi

  local timestamp out_file
  timestamp=$(date +%Y%m%d-%H%M%S)
  out_file="$backup_dir/pet-projects-$db-$timestamp.dump"

  if ! remote_exec "$vps_host" 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' > "$out_file"; then
    echo "backup.sh: pg_dump over SSH failed" >&2
    rm -f "$out_file"
    exit 1
  fi

  if [[ ! -s "$out_file" ]]; then
    echo "backup.sh: dump is empty, something went wrong upstream" >&2
    rm -f "$out_file"
    exit 1
  fi

  echo "backup saved: $out_file ($(du -h "$out_file" | cut -f1))"
  prune_old_backups "$backup_dir" "$db"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
