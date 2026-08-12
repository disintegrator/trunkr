#!/usr/bin/env bash
#MISE description="Apply `go fix` modernizers to the packages containing the given Go files"
set -euo pipefail

# hk passes individual .go files, but `go fix` operates on packages: map each
# file to its directory and fix each package once.
if [ "$#" -eq 0 ]; then
  exit 0
fi

for f in "$@"; do
  dirname "$f"
done | sort -u | while read -r dir; do
  go fix "./${dir#./}"
done