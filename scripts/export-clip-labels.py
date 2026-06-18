#!/usr/bin/env python3
"""Regenerate CLIP text embeddings from clip-labels.yaml.

This is a lightweight alternative to export-clip.py that only recomputes
the text embeddings without re-exporting the ONNX image encoder.

Usage:
    uv run --with transformers --with torch --with pyyaml --with numpy \
        python scripts/export-clip-labels.py
"""

import json
import os
import sys

import numpy as np
import torch
import yaml
from transformers import CLIPModel, CLIPTokenizerFast

MODEL_NAME = "openai/clip-vit-base-patch32"
OUTPUT_DIR = "models"
LABELS_PATH = os.path.join(OUTPUT_DIR, "clip-labels.json")
LABELS_YAML = os.path.join("scripts", "clip-labels.yaml")


def compute_text_embeddings(model, tokenizer, labels):
    """Compute L2-normalized text embeddings for each label."""
    print("Computing text embeddings...")

    results = []
    for label_def in labels:
        name = label_def["name"]
        prompt = label_def["prompt"]

        inputs = tokenizer(prompt, return_tensors="pt", padding=True, truncation=True)
        with torch.no_grad():
            text_outputs = model.text_model(**inputs)
            text_embeds = model.text_projection(text_outputs.pooler_output)

        embedding = text_embeds.squeeze(0).numpy()
        embedding = embedding / np.linalg.norm(embedding)

        entry = {
            "name": name,
            "prompt": prompt,
            "embedding": embedding.tolist(),
        }
        if label_def.get("group"):
            entry["group"] = label_def["group"]
        results.append(entry)
        print(f"  {name}: {prompt}")

    return results


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    if not os.path.exists(LABELS_YAML):
        print(f"Error: {LABELS_YAML} not found", file=sys.stderr)
        sys.exit(1)

    with open(LABELS_YAML) as f:
        label_config = yaml.safe_load(f)

    labels = label_config.get("labels", [])
    if not labels:
        print("Error: no labels defined", file=sys.stderr)
        sys.exit(1)

    print(f"Loading {MODEL_NAME} (text encoder only)...")
    model = CLIPModel.from_pretrained(MODEL_NAME)
    tokenizer = CLIPTokenizerFast.from_pretrained(MODEL_NAME)
    model.eval()

    label_embeddings = compute_text_embeddings(model, tokenizer, labels)

    dim = len(label_embeddings[0]["embedding"])

    output = {
        "model": "clip-vit-base-patch32",
        "dim": dim,
        "labels": label_embeddings,
    }

    with open(LABELS_PATH, "w") as f:
        json.dump(output, f, indent=2)

    print(f"Saved {len(label_embeddings)} label embeddings to {LABELS_PATH} (dim={dim})")


if __name__ == "__main__":
    main()
