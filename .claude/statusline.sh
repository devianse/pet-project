#!/usr/bin/env bash
# Claude Code statusline for pet-projects.
# Reads JSON on stdin, prints: repo | branch | context | 5h (+ reset) | 7d | model | effort

input=$(cat)

# --- colors -----------------------------------------------------------
RESET='\033[0m'
DIM='\033[2m'
CYAN='\033[36m'
GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
MAGENTA='\033[35m'
BLUE='\033[34m'
SEP="${DIM}|${RESET}"

# Pick a color for a 0-100 usage percentage: green/yellow/red thresholds.
color_for_pct() {
  local pct="$1"
  if [ -z "$pct" ]; then
    echo "$DIM"
  elif [ "$(LC_NUMERIC=C awk -v p="$pct" 'BEGIN{print (p>=80)}')" = "1" ]; then
    echo "$RED"
  elif [ "$(LC_NUMERIC=C awk -v p="$pct" 'BEGIN{print (p>=50)}')" = "1" ]; then
    echo "$YELLOW"
  else
    echo "$GREEN"
  fi
}

# --- repo ---------------------------------------------------------------
repo=$(echo "$input" | jq -r '.workspace.repo.name // empty')
cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // empty')
if [ -z "$repo" ] && [ -n "$cwd" ]; then
  repo=$(git -C "$cwd" --no-optional-locks rev-parse --show-toplevel 2>/dev/null | xargs -r basename)
fi
[ -z "$repo" ] && repo="no-repo"

# --- branch (worktree branch takes priority when in a worktree session) -
branch=$(echo "$input" | jq -r '.worktree.branch // empty')
if [ -z "$branch" ] && [ -n "$cwd" ]; then
  branch=$(git -C "$cwd" --no-optional-locks branch --show-current 2>/dev/null)
fi
[ -z "$branch" ] && branch="no-branch"

# --- context window usage ------------------------------------------------
ctx_used=$(echo "$input" | jq -r '.context_window.used_percentage // empty')
if [ -n "$ctx_used" ]; then
  ctx_color=$(color_for_pct "$ctx_used")
  context=$(LC_NUMERIC=C printf "ctx %.0f%%" "$ctx_used")
else
  ctx_color="$DIM"
  context="ctx n/a"
fi

# --- rate limits (Pro/Max plans only; absent otherwise) ------------------
five_h=$(echo "$input" | jq -r '.rate_limits.five_hour.used_percentage // empty')
five_h_resets_at=$(echo "$input" | jq -r '.rate_limits.five_hour.resets_at // empty')
seven_d=$(echo "$input" | jq -r '.rate_limits.seven_day.used_percentage // empty')

# Format a unix timestamp as "Xh Ym" (or "Ym") until reset, empty if absent/past.
format_reset() {
  local ts="$1"
  [ -z "$ts" ] && return
  local now secs
  now=$(date +%s)
  secs=$(( ts - now ))
  [ "$secs" -le 0 ] && return
  local h=$(( secs / 3600 ))
  local m=$(( (secs % 3600) / 60 ))
  if [ "$h" -gt 0 ]; then
    printf "%dh%02dm" "$h" "$m"
  else
    printf "%dm" "$m"
  fi
}

if [ -n "$five_h" ]; then
  five_h_color=$(color_for_pct "$five_h")
  five_h_reset=$(format_reset "$five_h_resets_at")
  if [ -n "$five_h_reset" ]; then
    five_h_str=$(LC_NUMERIC=C printf "5h %.0f%% (resets %s)" "$five_h" "$five_h_reset")
  else
    five_h_str=$(LC_NUMERIC=C printf "5h %.0f%%" "$five_h")
  fi
else
  five_h_color="$DIM"
  five_h_str="5h n/a"
fi

if [ -n "$seven_d" ]; then
  seven_d_color=$(color_for_pct "$seven_d")
  seven_d_str=$(LC_NUMERIC=C printf "7d %.0f%%" "$seven_d")
else
  seven_d_color="$DIM"
  seven_d_str="7d n/a"
fi

# --- model ----------------------------------------------------------------
model=$(echo "$input" | jq -r '.model.display_name // "unknown model"')

# --- effort/thinking level -------------------------------------------------
effort=$(echo "$input" | jq -r '.effort.level // empty')
thinking=$(echo "$input" | jq -r '.thinking.enabled // false')
if [ -n "$effort" ]; then
  effort_str="effort:$effort"
elif [ "$thinking" = "true" ]; then
  effort_str="thinking:on"
else
  effort_str="thinking:off"
fi

printf "${CYAN}%s${RESET} ${SEP} ${GREEN}%s${RESET} ${SEP} ${ctx_color}%s${RESET} ${SEP} ${five_h_color}%s${RESET} ${SEP} ${seven_d_color}%s${RESET} ${SEP} ${BLUE}%s${RESET} ${SEP} ${MAGENTA}%s${RESET}\n" \
  "$repo" "$branch" "$context" "$five_h_str" "$seven_d_str" "$model" "$effort_str"
