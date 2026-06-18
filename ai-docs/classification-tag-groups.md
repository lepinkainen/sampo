# Classification Tag Groups & Activating Embeddings

How CLIP classification tags work, why some labels are mutually exclusive, and
the exact steps to activate label/embedding changes.

## Tag groups (mutually-exclusive labels)

CLIP scores every label by cosine similarity against the image embedding and
keeps each label whose score is `>= classification.threshold` (config.yaml).
Without grouping, an image could get several overlapping tags from the same
concept — e.g. both `bikini` and `lingerie`.

Labels in `scripts/clip-labels.yaml` may declare a `group`. Within a group only
the **single highest-scoring** label is kept; ungrouped labels all pass through
independently.

```yaml
- name: bikini
  prompt: "a photo of a person wearing a bikini"
  group: attire
- name: lingerie
  prompt: "a photo of a person wearing lingerie or underwear"
  group: attire
- name: food
  prompt: "a photo of food or a meal"   # no group -> always independent
```

Current groups:

| Group      | Labels |
|------------|--------|
| `attire`   | bikini, swimsuit, nudity, lingerie, formal_wear, casual, sportswear, uniform, costume, winter_clothing, evening_dress |
| `location` | beach, pool, nature, urban, indoor |
| (none)     | food, animal, vehicle |

Result: an image gets at most one `attire` tag and at most one `location` tag,
plus any ungrouped tags above threshold.

Implementation: `internal/classification/classifier.go` (`Classify`, per-group
best-score selection) and the `Group` field on `Label`.

## Pipeline overview

```
scripts/clip-labels.yaml   (label names + prompts + groups; source of truth)
        |  export script (computes CLIP text embeddings)
        v
models/clip-labels.json    (embeddings + group, loaded by Go at startup)
        |  classifier compares image embedding vs label embeddings
        v
.cache/classification.db   (per-file cached tags, keyed by mtime/size/model_ver)
```

## Activating changes

Any edit to `scripts/clip-labels.yaml` (new label, changed prompt, group change)
requires three things to take effect: re-export embeddings, invalidate the
cache, re-scan.

### 1. Re-export embeddings

Text-only (fast, no ONNX re-export) — use after label/prompt/group edits:

```bash
uv run --with transformers --with torch --with pyyaml --with numpy \
    python scripts/export-clip-labels.py
# or: task download-clip-model   (also re-exports the image encoder)
```

This regenerates `models/clip-labels.json`, now including the `group` field.

Verify groups landed:

```bash
rg -o '"group": "[a-z]+"' models/clip-labels.json | sort | uniq -c
```

### 2. Invalidate the cache (bump model version)

`.cache/classification.db` caches tags per file. `Store.IsStale` treats a row as
stale when mtime, size, **or `model_version`** differs. Image files don't change,
so bump the version in `config.yaml` to force a full reclassify:

```yaml
classification:
  model_version: "clip-vit-b32-1.1"   # bump on any label/embedding change
```

(Alternative: delete `.cache/classification.db` to reclassify everything.)

### 3. Re-scan

Restart the backend, then trigger a scan per root.

**From the web UI:** the toolbar above the thumbnail grid has two scan buttons:

- ✨ *Classify images (CLIP)* — scans the **current directory level only** and
  skips files whose cached result is still fresh.
- 🔁 *Re-classify entire root from scratch* — recursive `force` scan of the whole
  root that ignores the cache and **replaces all existing tags**. Use this after
  changing labels, prompts, groups, or the model version. Prompts for
  confirmation first.

**Via the API** — `force` drives both recursion and cache-bypass:

```bash
# current dir only, skip fresh results
curl -s -X POST localhost:8080/api/classify/scan \
  -H 'Content-Type: application/json' \
  -d '{"rootId":"root-0","path":""}'

# full recursive rescan from scratch, replacing all results
curl -s -X POST localhost:8080/api/classify/scan \
  -H 'Content-Type: application/json' \
  -d '{"rootId":"root-0","path":"","force":true}'

curl -s localhost:8080/api/classify/status   # poll until running=false
```

When `force` is true the scan recurses into subdirectories and reclassifies
every image regardless of staleness; results are replaced via `Store.Put`
(DELETE old tags + INSERT OR REPLACE). `path` scopes the scan to a subtree
(`""` = whole root). Verify a file:

```bash
curl -s localhost:8080/api/classify/root-0/images/test_red.jpg
# tags should contain <=1 attire and <=1 location label;
# modelVer should match the bumped config value
```

## Display cap (separate from grouping)

The frontend shows only the top 3 tags (`tags.slice(0, 3)` in
`ThumbnailCard.svelte` and `ListView.svelte`); the backend stores all tags above
threshold. Grouping reduces the underlying tag count so the top-3 display is more
meaningful, but it is not what limits the display to three.
