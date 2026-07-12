#!/usr/bin/env bash
set -euo pipefail

# Enforce durable archive evidence. This intentionally scans historical archives
# rather than repairing them: a non-zero result identifies delivery drift that
# must be resolved in a dedicated follow-up change.
root_dir=$(git rev-parse --show-toplevel)
archive_dir="$root_dir/openspec/changes/archive"

if [[ ! -d "$archive_dir" ]]; then
  exit 0
fi

failures=0

fail() {
  printf 'openspec-delivery-check: %s\n' "$*" >&2
  failures=1
}

contains_unresolved_deferred() {
  local line=${1,,}
  [[ "$line" == *deferred* ]] && [[ ! "$line" =~ openspec/changes/[a-z0-9-]+ ]] && [[ ! "$line" =~ follow-up[[:space:]]+change:[[:space:]]*[a-z0-9-]+ ]]
}

shopt -s nullglob
for change_dir in "$archive_dir"/*; do
  [[ -d "$change_dir" ]] || continue
  change_name=${change_dir##*/}

  # Grandfather archives created before the acceptance gate (2026-07-12).
  # The gate enforces going forward; historical archives predate it and are
  # reconciled in the dedicated `reconcile-historical-archives` follow-up
  # change. Archive dirs are date-prefixed YYYY-MM-DD.
  gate_cutoff="2026-07-12"
  archive_date=${change_name:0:10}
  if [[ "$archive_date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] && [[ "$archive_date" < "$gate_cutoff" ]]; then
    printf 'openspec-delivery-check: %s grandfathered (pre-acceptance-gate); tracked for reconciliation\n' "$change_name" >&2
    continue
  fi

  tasks_file="$change_dir/tasks.md"

  if [[ ! -f "$tasks_file" ]]; then
    fail "$change_name has no tasks.md"
    continue
  fi

	last_task=''
	human_verification=0
  line_no=0
  while IFS= read -r line || [[ -n "$line" ]]; do
    ((line_no += 1))
    if [[ "$line" =~ ^[[:space:]]*-[[:space:]]+\[[[:space:]]\] ]]; then
      fail "$change_name/tasks.md:$line_no has an unchecked task"
    fi
    if [[ "$line" =~ ^[[:space:]]*-[[:space:]]+\[[xX]\] ]]; then
      last_task="$line"
      lower_line=${line,,}
      if [[ "$lower_line" == *"n/a"* || "$lower_line" == *"conditional"* ]] && [[ ! "$line" =~ N/A[[:space:]]because[[:space:]]no[[:space:]].+[[:space:]]files[[:space:]]changed ]]; then
        fail "$change_name/tasks.md:$line_no has conditional/N/A work without explicit 'N/A because no <surface> files changed' evidence"
      fi
    fi
  done < "$tasks_file"

	for note in "$change_dir"/*.md; do
    [[ -f "$note" ]] || continue
    note_line=0
    while IFS= read -r line || [[ -n "$line" ]]; do
      ((note_line += 1))
      lower_line=${line,,}
      if [[ "$lower_line" == *"agent-incapable"* ]]; then
        fail "$change_name/${note##*/}:$note_line contains prohibited agent-incapable completion note"
      fi
		if contains_unresolved_deferred "$line"; then
			fail "$change_name/${note##*/}:$note_line contains unresolved DEFERRED work; move it to a linked follow-up change"
		fi
		if [[ "$line" =~ ^[[:space:]]*Human[[:space:]]verification:[[:space:]][0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]]\|[[:space:]]irreducibly[[:space:]]external:[[:space:]].+\|[[:space:]]verified[[:space:]]by:[[:space:]].+ ]]; then
			human_verification=1
		fi
		done < "$note"
	done

	if [[ -z "$last_task" || "$last_task" != *"mise run acceptance"* ]] && [[ "$human_verification" -ne 1 ]]; then
		fail "$change_name must end with a checked acceptance task carrying exact command: mise run acceptance, or include 'Human verification: YYYY-MM-DD | irreducibly external: <check> | verified by: <person>'"
	fi
done

exit "$failures"
