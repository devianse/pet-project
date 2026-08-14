#!/usr/bin/env bash
# Unit tests for infra/backup.sh's local-only logic (prune_old_backups).
# Does not touch SSH or Docker — safe to run anywhere, anytime.
set -uo pipefail  # not -e: we want every assertion to run and report
# (sourcing backup.sh below re-enables -e via its own `set -euo
# pipefail`, since source runs in this same shell — turned back off
# right after the source so a failed assertion doesn't abort the run)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/backup.sh"
set +e

fail=0

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$expected" != "$actual" ]]; then
    echo "FAIL: $desc (expected [$expected], got [$actual])"
    fail=1
  else
    echo "PASS: $desc"
  fi
}

test_prune_keeps_most_recent_five() {
  local dir
  dir=$(mktemp -d)
  local i
  for i in 1 2 3 4 5 6 7; do
    touch "$dir/pet-projects-notes-2026010$i-000000.dump"
  done
  prune_old_backups "$dir" "notes" 5
  local remaining
  remaining=$(ls "$dir" | wc -l | tr -d ' ')
  assert_eq "keeps exactly 5 files" "5" "$remaining"
  local newest_present
  newest_present=$(ls "$dir" | grep -c '2026010[34567]')
  assert_eq "the 5 newest are the ones still present" "5" "$newest_present"
  rm -rf "$dir"
}

test_prune_noop_when_at_or_under_keep() {
  local dir
  dir=$(mktemp -d)
  touch "$dir/pet-projects-notes-20260101-000000.dump"
  touch "$dir/pet-projects-notes-20260102-000000.dump"
  prune_old_backups "$dir" "notes" 5
  local remaining
  remaining=$(ls "$dir" | wc -l | tr -d ' ')
  assert_eq "no pruning needed, both remain" "2" "$remaining"
  rm -rf "$dir"
}

test_prune_ignores_other_db_names() {
  local dir
  dir=$(mktemp -d)
  local i
  for i in 1 2 3 4 5 6; do
    touch "$dir/pet-projects-notes-2026010$i-000000.dump"
  done
  touch "$dir/pet-projects-watchlist-20260101-000000.dump"
  prune_old_backups "$dir" "notes" 5
  local remaining
  remaining=$(ls "$dir" | wc -l | tr -d ' ')
  assert_eq "prunes only the named db, leaves other dbs' dumps untouched" "6" "$remaining"
  rm -rf "$dir"
}

test_prune_keeps_most_recent_five
test_prune_noop_when_at_or_under_keep
test_prune_ignores_other_db_names

if [[ "$fail" -eq 1 ]]; then
  echo "SOME TESTS FAILED"
  exit 1
fi
echo "ALL TESTS PASSED"
