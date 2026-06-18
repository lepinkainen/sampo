# Sampo

Sampo is a local file and media browser with thumbnail previews, search, and optional AI-assisted image/video analysis.

It is built for browsing personal folders across multiple roots without moving files into a library.

## Features

- Browse multiple local folders from one web UI
- Preview images and videos with cached thumbnails
- Move, copy, rename, and delete files
- Search by filename and generated media tags
- Optional person detection and image classification

## Development

Requirements: Go, pnpm, Task, ffmpeg for video thumbnails.

```bash
cp config.example.yaml config.yaml
task dev-up
```

Open http://localhost:5173.

Useful commands:

```bash
task test
task build
task dev-down
```

AI features need local ONNX models:

```bash
task download-model
task download-clip-model
```
