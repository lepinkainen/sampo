# GEMINI.md

Sampo is a full-stack file manager with AI-powered media analysis capabilities, including person detection and image classification.

## Project Overview

- **Backend**: Go (v1.26+) using the `chi` router and `viper` for configuration.
- **Frontend**: Svelte 5 (SvelteKit) with TypeScript, Tailwind CSS 4, and Lucide icons.
- **AI/ML**: ONNX Runtime-based pipeline for person detection (YOLO11n) and image classification (CLIP ViT-B/32).
- **Architecture**: A two-process application where the Go backend serves a SvelteKit SPA from disk (static adapter).
- **Database**: SQLite is used for caching ML analysis results (stored in `.cache/`).

## Building and Running

The project uses `go-task` for automation. Key commands include:

### Development
- `task dev-up`: Start both backend (:8080) and frontend (:5173) in the background.
- `task dev-status`: Check health and status of background services.
- `task dev-down`: Stop all background development services.
- `task dev-logs`: Show paths to service logs (`.run/backend.log`, `.run/frontend.log`).
- `task dev`: Run both services in the foreground (requires manual management).

### Build & Test
- `task build`: Performs a full build (lint, test, frontend build, Go binary).
- `task test`: Runs Go unit tests and frontend type checking.
- `task test-full`: Runs unit tests, linting, builds, and Playwright E2E tests.
- `task test-e2e`: Runs Playwright E2E tests (requires services to be running).
- `task lint`: Lints both Go (golangci-lint) and Frontend (Biome) code.

### ML Setup
- `task download-model`: Downloads and exports the YOLO11n model to ONNX.
- `task download-clip-model`: Exports the CLIP model to ONNX and pre-computes text embeddings.

## Development Conventions

### Backend (Go)
- **Entry Point**: `cmd/sampo/main.go`.
- **Configuration**: Managed via `config.yaml` and `internal/config/config.go`.
- **Path Resolution**: All filesystem access must go through `internal/filesystem/roots.go` to ensure safety and prevent directory traversal.
- **Linting**: Standard Go tools (`goimports`, `go vet`) plus `golangci-lint`.

### Frontend (Svelte 5)
- **Location**: `frontend/` directory.
- **State Management**: Uses Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`).
- **Linting/Formatting**: Uses **Biome** instead of ESLint/Prettier. Run `task lint-frontend` or `cd frontend && pnpm exec biome check .`.
- **API**: Frontend proxies requests to the backend in development (see `vite.config.ts`).

### ML & Analysis
- Models are stored in the `models/` directory (ignored by git).
- Analysis results and thumbnails are cached in the `.cache/` directory.
- ONNX Runtime auto-detects the library path but can be overridden with `ORT_LIB_PATH`.

### Testing
- **Go**: Unit tests are located alongside source files (e.g., `*_test.go`).
- **E2E**: Playwright tests are located in `frontend/e2e/`.

## Key Files
- `Taskfile.yml`: Build and automation tasks.
- `CLAUDE.md`: High-level architectural guidance and command reference.
- `config.example.yaml`: Template for local configuration.
- `frontend/src/lib/api.ts`: Centralized API client for the frontend.
