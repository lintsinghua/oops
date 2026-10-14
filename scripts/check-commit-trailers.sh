#!/usr/bin/env bash
# Fail if any commit reachable from head but not from base has a message
# matching /co-authored-by/i.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
	echo "usage: scripts/check-commit-trailers.sh <base> [head]" >&2
	exit 2
fi

base=$1
head=${2:-HEAD}

if ! git rev-parse --verify --quiet "${base}^{commit}" >/dev/null; then
	echo "base not found: $base" >&2
	exit 2
fi
if ! git rev-parse --verify --quiet "${head}^{commit}" >/dev/null; then
	echo "head not found: $head" >&2
	exit 2
fi

bad=0
while IFS= read -r sha; do
	[[ -z $sha ]] && continue
	msg=$(git log -1 --format=%B "$sha")
	if printf '%s\n' "$msg" | grep -E -i -q 'co-authored-by'; then
		echo "commit $sha matches /co-authored-by/i:" >&2
		printf '%s\n' "$msg" >&2
		echo >&2
		bad=1
	fi
done < <(git rev-list "${base}..${head}")

if [[ $bad -ne 0 ]]; then
	echo "refusing Co-authored-by trailers in ${base}..${head}" >&2
	exit 1
fi
