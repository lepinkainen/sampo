# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
task build          # Full build: test + lint + frontend + Go binary
task test           # Run Go tests + frontend type checking
task test-full      # Full suite: test + lint + build + e2e (run after new features)
task test-e2e       # Playwright e2e tests only (requires dev servers running)
task lint           # goimports + go vet + golangci-lint
task build-frontend # Build SvelteKit frontend only
task build-go       # Build Go binary only (requires frontend/build to exist)
task dev            # Start frontend and backend in the foreground
task dev-up         # Start frontend and backend in the background via scripts/devctl
task dev-down       # Stop background dev services
task dev-status     # Show background dev service status as prettified JSON
task dev-restart    # Restart background dev services
task dev-logs       # Show background log file locations
task dev-frontend   # SvelteKit dev server (port 5173, proxies API to :8080)
task dev-go         # Go backend only (port 8080)
task clean          # Remove build artifacts
task download-model       # Download YOLOv8n and export to ONNX
task download-clip-model  # Export CLIP ViT-B/32 to ONNX + pre-compute text embeddings
```

Run a single Go test:
```bash
go test ./internal/filesystem/ -run TestResolvePath
```

Frontend is in `frontend/` — uses pnpm:
```bash
cd frontend && pnpm install && pnpm run build
```

Frontend linting uses Biome (not ESLint):
```bash
cd frontend && pnpm exec biome check .        # lint
cd frontend && pnpm exec biome check --write . # lint + autofix
```

E2E tests use Playwright (`frontend/e2e/`). Requires dev servers running:
```bash
task dev-up
cd frontend && pnpm exec playwright test
```

### Running during development

Use background dev services for development and testing:
```bash
task dev-up         # Start both backend (:8080) and frontend (:5173) in background
task dev-status     # Check if services are running and healthy
task dev-down       # Stop background services when done
task dev-restart    # Restart after code changes (rebuilds Go binary)
task dev-logs       # Show log file paths for debugging
```

- `dev-up` builds the Go binary, starts both services, and waits for health checks to pass
- Logs are written to `.run/backend.log` and `.run/frontend.log`
- If startup fails, the last 20 lines of the service log are printed automatically
- Always run `task dev-down` before exiting to avoid orphan processes

## Architecture

Two-process full-stack app: **Go backend** (chi router, port 8080) + **SvelteKit frontend** (static adapter, served from `frontend/build/` on disk).

### Backend (Go)

- **Entry point:** `cmd/sampo/main.go` — loads config, creates `os.DirFS("frontend/build")` for serving the SPA
- **Config:** `config.yaml` loaded via Viper — defines server port, cache dir, and filesystem roots
- **RootManager** (`internal/filesystem/roots.go`): manages multiple mounted directory roots. Each root gets an ID (`root-0`, `root-1`, etc.). All path resolution goes through `ResolvePath()` which prevents traversal via `filepath.Clean` + prefix checking + symlink resolution
- **Thumbnail pipeline** (`internal/thumbnail/`): on-demand generation with disk cache. Cache key = SHA256(rootID + path + mtime + size). Images use `disintegration/imaging` (JPEG output, 300px). Videos use ffmpeg exec
- **SPA serving** (`internal/server/routes.go`): tries to open requested file from `frontendFS`, falls back to `index.html` for client-side routing

### Frontend (SvelteKit + Svelte 5)

- Two-pane layout: `TreeView` (left) + `ThumbnailGrid` (right)
- Uses Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`)
- `frontend/src/lib/api.ts` wraps all fetch calls; `vite.config.ts` proxies `/api`, `/whoami`, `/health` to `:8080` in dev mode
- Static adapter outputs to `frontend/build/` — no SSR, pure SPA

### ML Pipeline (ONNX-based)

Both detection and classification are optional — enabled via `config.yaml` `detection:` and `classification:` sections. Models live in `models/` (gitignored); download with `task download-model` / `task download-clip-model`.

- **ONNX Runtime** (`internal/onnxenv/`): shared lazy init via `sync.Once`. Auto-detects library path per platform (Homebrew on ARM64 macOS, `/usr/lib` on Linux). Override with `ORT_LIB_PATH` env var. Uses CoreML execution provider on macOS
- **Person detection** (`internal/detection/`): YOLOv8n model. Scanner for batch directory processing, SQLite-backed result cache
- **Image classification** (`internal/classification/`): CLIP ViT-B/32 vision encoder. Text embeddings pre-computed at export time from `scripts/clip-labels.yaml` (18 labels: clothing, locations, etc.). Scanner + SQLite store. Classifies by cosine similarity against label embeddings

### API

| Endpoint | Purpose |
|----------|---------|
| `GET /api/roots` | List configured roots |
| `GET /api/tree/{rootID}/*path` | Directory listing (one level, lazy) |
| `GET /api/thumb/{rootID}/*path` | Thumbnail (generates on first request, then cached) |
| `GET /api/file/{rootID}/*path` | Serve original file with Content-Type and Range support |
| `DELETE /api/files/{rootID}/*path` | Delete file or directory (`?recursive=true` for non-empty dirs) |
| `POST /api/files/move` | Move/rename files (supports cross-root, smart dedup) |
| `POST /api/files/copy` | Copy files (supports cross-root, smart dedup) |
| `GET /api/detect/{rootID}/*path` | Run person detection on single image |
| `POST /api/detect/scan` | Start background detection scan |
| `GET /api/detect/status` | Poll detection scan progress |
| `GET /api/classify/{rootID}/*path` | Classify single image (or get cached result) |
| `POST /api/classify/scan` | Start background classification scan |
| `GET /api/classify/status` | Poll classification scan progress |
| `GET /api/search/{rootID}?q=<query>&path=<scope>` | Search files by name and classification tags |
| `GET /api/usage/{rootID}/*path` | Disk usage stats (total size, file/dir counts) |
| `GET /api/duplicates/{rootID}/*path` | Find duplicate files by SHA256 checksum |
| `GET /whoami` | App version info |
| `GET /health` | Health check |

## Key Conventions

- Frontend is **not embedded** in the Go binary — served from disk via `os.DirFS`. This avoids `go:embed` issues with `_app/` directories
- Hidden files (dotfiles) are excluded from directory listings
- `llm-shared/` is a git submodule — **do not edit**. Excluded from linting and testing
- Thumbnail cache lives in `.cache/thumbs/{rootID}/` relative to working directory
- ONNX Runtime library must be installed on the system; auto-detected per platform or override with `ORT_LIB_PATH`
- ML results cached in SQLite databases under `.cache/`
- Version injected via ldflags: `-X main.version=...`
