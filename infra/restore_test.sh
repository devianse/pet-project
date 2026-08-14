#!/usr/bin/env bash
# Tests for infra/restore.sh.
#
# Part 1 (always runs): argument validation, no Docker needed.
# Part 2 (integration): spins up a throwaway Postgres container,
# dumps a seeded row, wipes it, restores via restore.sh, and checks
# the row comes back. Requires Docker running locally — same
# requirement as the rest of local dev, see
# CHEATSHEET.local/02-postgres-db.md. Skips itself if Docker isn't
# available rather than failing.
set -uo pipefail  # not -e: we want every assertion to run and report

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail=0

assert_contains() {
  local desc="$1" haystack="$2" needle="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "FAIL: $desc"
    echo "  expected output to contain: $needle"
    echo "  actual output: $haystack"
    fail=1
  else
    echo "PASS: $desc"
  fi
}

assert_nonzero_exit() {
  local desc="$1" code="$2"
  if [[ "$code" -eq 0 ]]; then
    echo "FAIL: $desc (expected nonzero exit, got 0)"
    fail=1
  else
    echo "PASS: $desc"
  fi
}

# --- Part 1: argument validation ---

out=$("$SCRIPT_DIR/restore.sh" 2>&1); code=$?
assert_contains "no args -> usage message" "$out" "Usage:"
assert_nonzero_exit "no args -> nonzero exit" "$code"

out=$("$SCRIPT_DIR/restore.sh" /no/such/file.dump 2>&1); code=$?
assert_contains "missing file -> error message" "$out" "no such file"
assert_nonzero_exit "missing file -> nonzero exit" "$code"

empty_file=$(mktemp)
out=$("$SCRIPT_DIR/restore.sh" "$empty_file" 2>&1); code=$?
assert_contains "empty file -> error message" "$out" "empty"
assert_nonzero_exit "empty file -> nonzero exit" "$code"
rm -f "$empty_file"

if ! command -v docker >/dev/null 2>&1; then
  echo "SKIP: Docker not available, skipping 'no running postgres service' test"
else
  no_compose_dir=$(mktemp -d)
  valid_dump=$(mktemp --suffix=.dump)
  echo "not empty" > "$valid_dump"
  out=$(cd "$no_compose_dir" && "$SCRIPT_DIR/restore.sh" "$valid_dump" 2>&1); code=$?
  assert_contains "no container arg, no compose file -> friendly error" "$out" "no running"
  assert_nonzero_exit "no container arg, no compose file -> nonzero exit" "$code"
  rm -f "$valid_dump"
  rmdir "$no_compose_dir"
fi

# --- Part 2: integration round-trip (requires Docker) ---

if ! command -v docker >/dev/null 2>&1; then
  echo "SKIP: Docker not available, skipping integration round-trip test"
else
  container="restore-test-$$"
  docker run -d --name "$container" \
    -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=notes \
    postgres:16-alpine >/dev/null

  cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
  trap cleanup EXIT

  ready=0
  for _ in $(seq 1 30); do
    if docker exec "$container" pg_isready -U postgres -d notes >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  if [[ "$ready" -ne 1 ]]; then
    echo "FAIL: test container never became ready"
    fail=1
  else
    docker exec "$container" psql -U postgres -d notes -c \
      "CREATE TABLE restore_marker (id serial primary key, note text);" >/dev/null
    docker exec "$container" psql -U postgres -d notes -c \
      "INSERT INTO restore_marker (note) VALUES ('round-trip-ok');" >/dev/null

    dump_file=$(mktemp --suffix=.dump)
    docker exec "$container" sh -c 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' > "$dump_file"

    docker exec "$container" psql -U postgres -d notes -c \
      "DROP TABLE restore_marker;" >/dev/null

    "$SCRIPT_DIR/restore.sh" "$dump_file" "$container" >/dev/null

    result=$(docker exec "$container" psql -U postgres -d notes -tAc \
      "SELECT note FROM restore_marker;")
    assert_contains "restored row matches what was dumped" "$result" "round-trip-ok"

    rm -f "$dump_file"
  fi
fi

if [[ "$fail" -eq 1 ]]; then
  echo "SOME TESTS FAILED"
  exit 1
fi
echo "ALL TESTS PASSED"
