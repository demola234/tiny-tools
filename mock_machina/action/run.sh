#!/usr/bin/env bash
set -euo pipefail

mockmachina="${MOCKMACHINA:-mockmachina}"
dir="${DIR:-.mockmachina}"
override="${OVERRIDE_LABEL:-breaking-change}"
marker='<!-- mockmachina-diff -->'

"$mockmachina" lint --dir "$dir"

if [ -z "${BASE_REF:-}" ]; then
  echo "not a pull request, so only lint ran" >&2
  exit 0
fi

if ! base=$(git merge-base "origin/$BASE_REF" HEAD); then
  echo "can't find where this branch left $BASE_REF; check out with fetch-depth: 0" >&2
  exit 1
fi
range="$base..HEAD"

"$mockmachina" diff "$range" --dir "$dir" --format github
report=$("$mockmachina" diff "$range" --dir "$dir" --format markdown)

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  printf '%s\n' "$report" >> "$GITHUB_STEP_SUMMARY"
fi

comment() {
  local comments="repos/$GITHUB_REPOSITORY/issues/$PR_NUMBER/comments"
  local existing
  existing=$(gh api -X GET "$comments" --paginate --jq ".[] | select(.body | startswith(\"$marker\")) | .id" | head -n 1)
  if [ -n "$existing" ]; then
    gh api -X PATCH "repos/$GITHUB_REPOSITORY/issues/comments/$existing" -f body="$marker
$report" > /dev/null
  elif ! grep -q '^No contract changes' <<< "$report"; then
    gh api -X POST "$comments" -f body="$marker
$report" > /dev/null
  fi
}

if [ "${COMMENT:-true}" = true ] && [ -n "${PR_NUMBER:-}" ]; then
  comment || echo "::warning::couldn't comment on the pull request (pull requests from forks can't be commented on); the report is in the job summary"
fi

if ! "$mockmachina" diff "$range" --dir "$dir" --fail-on breaking > /dev/null; then
  if [[ ",${LABELS:-}," == *",$override,"* ]]; then
    echo "breaking contract changes allowed by the $override label" >&2
  else
    echo "breaking contract changes; add the $override label to the pull request if they're intended" >&2
    exit 1
  fi
fi
