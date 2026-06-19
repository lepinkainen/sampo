# Running Sampo on unraid

The GHCR image `ghcr.io/lepinkainen/sampo:latest` is self-contained: ML models
(YOLO11n + CLIP ViT-B/32) are baked in, so no model mounts are needed.

## What you need to mount

| Container path     | unraid host path                     | Mode | Notes |
|--------------------|--------------------------------------|------|-------|
| `/cache`           | `/mnt/user/appdata/sampo/cache`      | rw   | Thumbnails + ML SQLite. Must be writable by the container user. |
| `/app/config.yaml` | `/mnt/user/appdata/sampo/config.yaml`| ro   | Your multi-root config (overrides baked one). |
| `/data/<name>`     | `/mnt/user/<Share>`                  | ro   | One per share you want to browse. |

Port: container `8080` → any host port (e.g. `8086`).

## Run as 99:100 (recommended)

The image runs as a fixed user **uid 1000** by default — there is no
`PUID`/`PGID` support. Rather than chmod'ing the cache dir, override the user at
runtime to unraid's standard `nobody:users` (99:100):

- **Add Container UI:** toggle **BASIC VIEW → ADVANCED VIEW** (top right), then
  set **Extra Parameters** to `--user 99:100`.
- **docker run:** add `--user 99:100`.

This works because the image's files (`/app/sampo`, `/app/models`, the mounted
`config.yaml`) are world-readable/executable, and `/cache` ownership comes from
the host appdata dir (default `nobody:users`).

Create the cache dir owned 99:100:

```sh
mkdir -p /mnt/user/appdata/sampo/cache
chown -R 99:100 /mnt/user/appdata/sampo/cache
```

## config.yaml must exist as a file before first run

Bind-mounting a host path that doesn't exist makes Docker create it as a
**directory**, which then fails to mount onto the `/app/config.yaml` file
("cannot create subdirectories ... not a directory"). Create the file first:

```sh
nano /mnt/user/appdata/sampo/config.yaml   # paste config, Ctrl-O, Ctrl-X
ls -l /mnt/user/appdata/sampo/config.yaml  # must be a file, not a dir
```

Use `config.yaml` in this folder as the template. Each `roots:` `path:` must
match a `/data/<name>` container path you map. Photo shares mount `ro`.

## Setup steps (Add Container UI)

1. Create the cache dir + config file (above).
2. Docker tab → **Add Container**.
3. Name `Sampo`, Repository `ghcr.io/lepinkainen/sampo:latest`, Network Bridge.
4. Add a **Port**: container `8080` → host `8086`.
5. Add **Path** mappings from the table above (Cache rw, Config ro, one share
   per root ro).
6. **ADVANCED VIEW** → Extra Parameters `--user 99:100`.
7. **ADVANCED VIEW** → **WebUI** → `http://[IP]:[PORT:8080]` so the unraid
   container's WebUI link opens the right port (see below).
8. Apply, then open `http://<unraid-ip>:<host-port>/`.

## WebUI link (unraid container icon)

The **WebUI** field (ADVANCED VIEW) sets where unraid's "WebUI" container menu
points. Use the `[PORT:nnnn]` token so it follows your published host port:

```
http://[IP]:[PORT:8080]
```

`[IP]` resolves to the server, `[PORT:8080]` resolves to whatever host port you
mapped container `8080` to. Pick a free host port — `8080` and the common
alternates may already be taken by other containers; e.g. `8089` works:

```
http://[IP]:[PORT:8089]
```

with the Port mapping host `8089` → container `8080`.

## Container icon

The **Icon URL** field (ADVANCED VIEW) takes a publicly reachable image URL.
Point it at the logo in the repo:

```
https://raw.githubusercontent.com/lepinkainen/sampo/main/assets/branding/sampo-logo.svg
```

## Multiple roots

Each entry under `roots:` becomes a browse root (root-0, root-1, … in listed
order). Add a matching Docker Path mapping per root. To add another share: add a
`roots:` entry with `path: /data/foo` and a Path mapping `/data/foo` →
`/mnt/user/Foo`.

## Updating

Force-update the container in the unraid Docker tab (or `docker pull` + recreate).
Baked models update with the image; your `config.yaml` and `/cache` persist.

## Troubleshooting

- **`API version [26] is not available ... only [1, 24]`** — the baked ONNX
  Runtime lib is older than the Go binding expects. Fixed by pinning
  `ORT_VERSION` in the Dockerfile to match `onnxruntime_go` (1.26.0 → API 26).
  Pull a fresh image.
- **`no roots configured`** — `config.yaml` `roots:` is empty or wasn't mounted
  (check it's a file, not a dir).
