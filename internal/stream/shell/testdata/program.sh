#!/usr/bin/env bash
# A typical script.
set -euo pipefail

readonly DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION=1.2.3
count=0

usage() {
  echo "usage: $0 [-v|--verbose] <file>..." >&2
  exit 1
}

function log {
  local level=$1; shift
  printf '%s [%s] %s\n' "$(date +%T)" "$level" "$*"
}

for f in "$@"; do
  if [[ -f $f && ! -L "$f" ]]; then
    count=$((count + 1))
    grep -q 'pattern' "$f" || continue
  elif [ -d "$f" ]; then
    find "$f" -name '*.txt' -exec cat {} \;
  else
    log WARN "skipping $f"
  fi
done

case "$1" in
  start|stop) service app "$1" ;;
  *) usage ;;
esac

while read -r line; do
  echo "${line%%:*}" | cut -d: -f2-
done < /etc/passwd

until false; do sleep 1; done
export PATH="$HOME/bin:$PATH" EDITOR=vim
x=`ls -la | wc -l`
arr=(one "two three" 'four')
echo ${#arr[@]} ${arr[1]} $? $$ $! $1 $10 $@
sudo make install 2>&1 | tee build.log
#!/bin/sh -e
x=1 #!inline
