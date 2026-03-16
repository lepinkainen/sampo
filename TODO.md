# TODO

## Core Features
- [ ] ZIP file browsing as virtual directories in tree
- [ ] Video thumbnails via ffmpeg
- [ ] Virtual scrolling for large directories
- [ ] Single-binary distribution via `//go:embed all:frontend`

## Infrastructure
- [ ] Dockerfile (alpine + ffmpeg)
- [ ] docker-compose.yml with volume mounts
- [ ] .dockerignore
- [ ] GitHub Actions CI

## Enhancements
- [ ] SQLite metadata cache
- [x] File detail panel (dimensions, duration, EXIF, etc.)
- [ ] Thumbnail pre-generation for browsed directories
- [x] Multiple thumbnail sizes (small grid, medium preview)
- [ ] Keyboard navigation in tree and grid
- [ ] Search/filter within current directory
- [ ] Background thumbnail worker pool

## Future / Nice to Have
- [ ] CLIP embeddings for similarity search
- [ ] YOLO/LLM content analysis
- [ ] Drag-and-drop file organization
- [ ] Configurable per-root settings (read-only, thumbnail policy)
