"""Shared loading for CLIP label YAML files.

Tracked labels live in clip-labels.yaml. Optional private labels live in
clip-labels.local.yaml and are merged when present.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import yaml

SCRIPT_DIR = Path(__file__).resolve().parent
BASE_LABELS_YAML = SCRIPT_DIR / "clip-labels.yaml"
DEFAULT_LOCAL_LABELS_YAML = SCRIPT_DIR / "clip-labels.local.yaml"


def _load_yaml(path: Path) -> dict[str, Any]:
    with path.open() as f:
        data = yaml.safe_load(f) or {}
    if not isinstance(data, dict):
        raise ValueError(f"{path} must contain a YAML mapping")
    return data


def _labels_from(path: Path) -> list[dict[str, Any]]:
    data = _load_yaml(path)
    labels = data.get("labels", [])
    if not isinstance(labels, list):
        raise ValueError(f"{path} labels must be a list")
    for i, label in enumerate(labels, start=1):
        if not isinstance(label, dict):
            raise ValueError(f"{path} label #{i} must be a mapping")
        for key in ("name", "prompt"):
            if not label.get(key):
                raise ValueError(f"{path} label #{i} missing {key!r}")
    return labels


def _local_labels_path() -> Path | None:
    raw = os.environ.get("CLIP_LABELS_LOCAL")
    if raw is None:
        return DEFAULT_LOCAL_LABELS_YAML
    if raw.lower() in {"", "0", "false", "no", "none"}:
        return None
    return Path(raw)


def load_label_definitions() -> list[dict[str, Any]]:
    """Load base labels, then optional local labels.

    Local labels are appended. If a local label reuses a base label name, it
    replaces that base label in-place so private prompts/groups can override
    tracked defaults without duplicate output tags.
    """
    if not BASE_LABELS_YAML.exists():
        raise FileNotFoundError(f"{BASE_LABELS_YAML} not found")

    sources = [(BASE_LABELS_YAML, _labels_from(BASE_LABELS_YAML))]
    local_path = _local_labels_path()
    if local_path is not None and local_path.exists():
        sources.append((local_path, _labels_from(local_path)))

    labels: list[dict[str, Any]] = []
    positions: dict[str, int] = {}
    for _, source_labels in sources:
        for label in source_labels:
            name = label["name"]
            if name in positions:
                labels[positions[name]] = label
                continue
            positions[name] = len(labels)
            labels.append(label)

    if not labels:
        raise ValueError("no labels defined")
    return labels
