#!/usr/bin/env bash
# Checks that the tools needed for canary-controller are installed.
# Exits non-zero if a required tool is missing or Docker isn't running.

set -u

missing=0

# check <required|optional> <command> <version command...>
check() {
  local kind="$1" name="$2"
  shift 2
  if command -v "$name" >/dev/null 2>&1; then
    printf '  %-9s %s\n' "$name" "$("$@" 2>/dev/null | head -n 1)"
  elif [ "$kind" = "required" ]; then
    printf '  %-9s MISSING\n' "$name"
    missing=1
  else
    printf '  %-9s not installed (optional for now)\n' "$name"
  fi
}

echo "Required:"
check required go      go version
check required docker  docker --version
check required kind    kind version
check required kubectl kubectl version --client
check required helm    helm version --short
check required k6      k6 version

echo "Later or optional:"
check optional gcloud  gcloud --version
check optional k9s     k9s version --short
check optional mongosh mongosh --version
check optional redis-cli redis-cli --version

echo "Docker:"
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  mem_bytes="$(docker info --format '{{.MemTotal}}' 2>/dev/null || echo 0)"
  mem_gb=$(( mem_bytes / 1024 / 1024 / 1024 ))
  echo "  daemon running, ${mem_gb} GB memory available"
  if [ "$mem_gb" -lt 6 ]; then
    echo "  WARNING: give Docker at least 6 GB of memory for Phase 2 onwards"
  fi
else
  echo "  daemon NOT running (start Docker Desktop or your Docker engine)"
  missing=1
fi

if [ "$missing" -ne 0 ]; then
  echo
  echo "Some required tools are missing. See docs/setup.md."
  exit 1
fi

echo
echo "All required tools found."
