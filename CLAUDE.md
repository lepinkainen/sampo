# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
task build          # Full build: test + lint + frontend + Go binary
task test           # Run Go tests (excludes llm-shared)
task lint           # goimports + go vet + golangci-lint
task build-frontend # Build SvelteKit frontend only
task build-go       # Build Go binary only (requires frontend/build to exist)
task dev            # Start both frontend dev server and Go backend
task dev-frontend   # SvelteKit dev server (port 5173, proxies API to :8080)
task dev-go         # Go backend only (port 8080)
task clean          # Remove build artifacts
```

Run a single Go test:
```bash
go test ./internal/filesystem/ -run TestResolvePath
```

Frontend is in `frontend/` — uses npm (not pnpm):
```bash
cd frontend && npm install && npm run build
```

## Architecture

Two-process full-stack app: **Go backend** (chi router, port 8080) + **SvelteKit frontend** (static adapter, served from `frontend/build/` on disk).

### Backend (Go)

- **Entry point:** `cmd/filemanager/main.go` — loads config, creates `os.DirFS("frontend/build")` for serving the SPA
- **Config:** `config.yaml` loaded via Viper — defines server port, cache dir, and filesystem roots
- **RootManager** (`internal/filesystem/roots.go`): manages multiple mounted directory roots. Each root gets an ID (`root-0`, `root-1`, etc.). All path resolution goes through `ResolvePath()` which prevents traversal via `filepath.Clean` + prefix checking + symlink resolution
- **Thumbnail pipeline** (`internal/thumbnail/`): on-demand generation with disk cache. Cache key = SHA256(rootID + path + mtime + size). Images use `disintegration/imaging` (JPEG output, 300px). Videos use ffmpeg exec
- **SPA serving** (`internal/server/routes.go`): tries to open requested file from `frontendFS`, falls back to `index.html` for client-side routing

### Frontend (SvelteKit + Svelte 5)

- Two-pane layout: `TreeView` (left) + `ThumbnailGrid` (right)
- Uses Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`)
- `frontend/src/lib/api.ts` wraps all fetch calls; `vite.config.ts` proxies `/api`, `/whoami`, `/health` to `:8080` in dev mode
- Static adapter outputs to `frontend/build/` — no SSR, pure SPA

### API

| Endpoint | Purpose |
|----------|---------|
| `GET /api/roots` | List configured roots |
| `GET /api/tree/{rootID}/*path` | Directory listing (one level, lazy) |
| `GET /api/thumb/{rootID}/*path` | Thumbnail (generates on first request, then cached) |
| `GET /whoami` | App version info |
| `GET /health` | Health check |

## Key Conventions

- Frontend is **not embedded** in the Go binary — served from disk via `os.DirFS`. This avoids `go:embed` issues with `_app/` directories
- Hidden files (dotfiles) are excluded from directory listings
- `llm-shared/` is a git submodule — **do not edit**. Excluded from linting and testing
- Thumbnail cache lives in `.cache/thumbs/{rootID}/` relative to working directory
- Version injected via ldflags: `-X main.version=...`
