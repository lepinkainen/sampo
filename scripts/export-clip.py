#!/usr/bin/env python3
"""Export CLIP ViT-B/32 image encoder to ONNX and pre-compute text embeddings.

Loads scripts/clip-labels.yaml plus optional private
scripts/clip-labels.local.yaml when present.

Usage:
    uv run --with transformers --with torch --with onnx --with onnxruntime --with pyyaml \
        python scripts/export-clip.py

Outputs:
    models/clip-vit-b32-image.onnx   — CLIP image encoder
    models/clip-labels.json          — Pre-computed label embeddings
"""

import json
import os

import numpy as np
import onnx
import torch
from clip_labels import load_label_definitions
from transformers import CLIPModel, CLIPTokenizerFast

MODEL_NAME = "openai/clip-vit-base-patch32"
OUTPUT_DIR = "models"
ONNX_PATH = os.path.join(OUTPUT_DIR, "clip-vit-b32-image.onnx")
LABELS_PATH = os.path.join(OUTPUT_DIR, "clip-labels.json")


def export_image_encoder(model):
    """Export the CLIP image encoder (vision model + visual projection) to ONNX."""
    print("Exporting CLIP image encoder to ONNX...")

    dummy_image = torch.randn(1, 3, 224, 224)

    class CLIPImageEncoder(torch.nn.Module):
        def __init__(self, clip_model):
            super().__init__()
            self.vision_model = clip_model.vision_model
            self.visual_projection = clip_model.visual_projection

        def forward(self, pixel_values):
            vision_outputs = self.vision_model(pixel_values=pixel_values)
            image_embeds = self.visual_projection(vision_outputs.pooler_output)
            return image_embeds

    encoder = CLIPImageEncoder(model)
    encoder.eval()

    torch.onnx.export(
        encoder,
        (dummy_image,),
        ONNX_PATH,
        input_names=["pixel_values"],
        output_names=["image_embeds"],
        dynamic_axes={
            "pixel_values": {0: "batch_size"},
            "image_embeds": {0: "batch_size"},
        },
        opset_version=18,
    )

    # The dynamo exporter may produce external data files.
    # Inline all weights into a single .onnx protobuf so ONNX Runtime
    # can load the model without needing to resolve external paths.
    external_data = ONNX_PATH + ".data"
    if os.path.exists(external_data):
        print("  Inlining external weights into single .onnx file...")
        model_proto = onnx.load(ONNX_PATH, load_external_data=True)
        onnx.save_model(
            model_proto,
            ONNX_PATH,
            save_as_external_data=False,
        )
        os.remove(external_data)

    print(f"  Saved to {ONNX_PATH}")


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
            # Project pooler output through text projection
            text_embeds = model.text_projection(text_outputs.pooler_output)

        # text_embeds is [1, dim] — squeeze to 1D and L2 normalize
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

    # Load label definitions
    labels = load_label_definitions()

    print(f"Loading {MODEL_NAME}...")
    model = CLIPModel.from_pretrained(MODEL_NAME)
    tokenizer = CLIPTokenizerFast.from_pretrained(MODEL_NAME)
    model.eval()

    # Export image encoder
    export_image_encoder(model)

    # Compute text embeddings
    label_embeddings = compute_text_embeddings(model, tokenizer, labels)

    # Get embedding dimension
    dim = len(label_embeddings[0]["embedding"])

    # Save labels JSON
    output = {
        "model": "clip-vit-base-patch32",
        "dim": dim,
        "logitScale": model.logit_scale.exp().item(),
        "labels": label_embeddings,
    }

    with open(LABELS_PATH, "w") as f:
        json.dump(output, f, indent=2)

    print(f"Saved {len(label_embeddings)} label embeddings to {LABELS_PATH} (dim={dim})")


if __name__ == "__main__":
    main()
