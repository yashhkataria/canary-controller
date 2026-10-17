# Setup

Everything here runs locally for free. Only Phase 9 (GKE) touches the cloud.

## 1. Install the tools

| Tool | Needed from | macOS (Homebrew) | Linux / Windows |
| --- | --- | --- | --- |
| Go (latest stable) | Phase 1 | `brew install go` | Installer from go.dev/dl |
| Docker | Phase 1 | Docker Desktop from docker.com (or Colima) | Docker Engine on Linux; Docker Desktop with WSL2 on Windows |
| kind | Phase 2 | `brew install kind` | kind's Quick Start page, "Installation" |
| kubectl | Phase 2 | `brew install kubectl` | kubernetes.io, "Install Tools" |
| Helm | Phase 2 | `brew install helm` | helm.sh, "Installing Helm" |
| k6 | Phase 2 | `brew install k6` | Grafana k6 docs, "Install k6" |
| gcloud CLI | Phase 9 | Install later | Install later |
| k9s, mongosh, redis-cli | Optional | `brew install k9s` etc. | Optional |

On Windows, do everything inside WSL2 (Ubuntu) and use Docker Desktop's WSL integration.

Give Docker at least 6 GB of memory (Docker Desktop → Settings → Resources). Prometheus, Grafana, MongoDB, Redis and 10 demo pods add up.

## 2. Check everything is installed

```bash
make tools
```

This runs `scripts/check-tools.sh`, which prints each tool's version, flags anything missing, checks that Docker is running, and warns if Docker has less than 6 GB of memory. It exits non-zero if a required tool is missing.

## 3. Prepare the repo

From the repo root:

```bash
# Only if go.mod doesn't exist yet; match your GitHub repo name
go mod init github.com/yashhkataria/canary-controller

chmod +x scripts/check-tools.sh
git add .
git commit -m "Phase 0: project setup"
git tag phase-0
git push -u origin main --tags
```

If your repo name isn't `canary-controller`, update the module path in `CLAUDE.md` to match.

## 4. Start Phase 1 in Claude Code

Open Claude Code in the repo root and paste:

```text
Read CLAUDE.md and docs/design.md. The repo layout already exists, along with go.mod, CLAUDE.md and the docs. Add a
`build-demo` Makefile target as part of this work, then build cmd/demo-app:
an HTTP server on :8080 with GET / (JSON with version and track),
GET /healthz and GET /metrics.
- Read VERSION, TRACK, ERROR_RATE (0 to 1) and EXTRA_LATENCY_MS from env
  vars. ERROR_RATE makes that fraction of requests return 500;
  EXTRA_LATENCY_MS adds a delay to every request.
- Record http_requests_total{track, code} and the histogram
  http_request_duration_seconds{track} with prometheus/client_golang.
- Graceful shutdown on SIGTERM (http.Server.Shutdown with a timeout).
- Multi-stage Dockerfile producing a small non-root image. VERSION,
  ERROR_RATE and EXTRA_LATENCY_MS default from build args, so we can build
  demo:v1, demo:v2, demo:v2-bad (20% errors) and demo:v2-slow (+800 ms).
  The build-demo target builds all four.
- Unit tests for error injection and metric labels.
Propose a plan before writing code.
```

When the phase is done: check the "Done when" list in the build plan, commit, `git tag phase-1`, write your entry in `docs/notes.md`, and update the Status lines in `CLAUDE.md`.
