#!/usr/bin/env python3
"""Generate deterministic 2x2 RGB colour-grid image fixtures.

Outputs 81 PNGs by default: every red/green/blue assignment for the four
quadrants. File names use the common prefix `grid2x2_rgb_`; the four suffix
letters encode top-left, top-right, bottom-left, bottom-right.
"""

import argparse
import itertools
import os
import struct
import zlib
from pathlib import Path

PREFIX = "grid2x2_rgb_"
COLOURS = {
    "r": (255, 0, 0),
    "g": (0, 255, 0),
    "b": (0, 0, 255),
}


def png_chunk(kind: bytes, data: bytes) -> bytes:
    return (
        struct.pack(">I", len(data))
        + kind
        + data
        + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    )


def write_png(path: Path, pixels: list[list[tuple[int, int, int]]]) -> None:
    height = len(pixels)
    width = len(pixels[0])

    raw_rows = bytearray()
    for row in pixels:
        raw_rows.append(0)  # PNG filter: none.
        for red, green, blue in row:
            raw_rows.extend((red, green, blue))

    ihdr = struct.pack(
        ">IIBBBBB",
        width,
        height,
        8,  # bit depth
        2,  # truecolour RGB
        0,  # compression method
        0,  # filter method
        0,  # no interlace
    )

    path.write_bytes(
        b"\x89PNG\r\n\x1a\n"
        + png_chunk(b"IHDR", ihdr)
        + png_chunk(b"IDAT", zlib.compress(bytes(raw_rows), level=9))
        + png_chunk(b"IEND", b"")
    )


def grid_pixels(pattern: tuple[str, str, str, str], size: int) -> list[list[tuple[int, int, int]]]:
    half = size // 2
    top_left, top_right, bottom_left, bottom_right = (COLOURS[c] for c in pattern)

    pixels = []
    for y in range(size):
        row = []
        for x in range(size):
            if y < half:
                row.append(top_left if x < half else top_right)
            else:
                row.append(bottom_left if x < half else bottom_right)
        pixels.append(row)
    return pixels


def generate(output_dir: Path, size: int) -> int:
    if size < 2 or size % 2 != 0:
        raise ValueError("size must be an even integer >= 2")

    output_dir.mkdir(parents=True, exist_ok=True)
    for old_file in output_dir.glob(f"{PREFIX}*.png"):
        old_file.unlink()

    count = 0
    for pattern in itertools.product(COLOURS.keys(), repeat=4):
        suffix = "".join(pattern)
        out_path = output_dir / f"{PREFIX}{suffix}.png"
        write_png(out_path, grid_pixels(pattern, size))
        count += 1
    return count


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("testdata/grid2x2"),
        help="directory for generated PNG fixtures",
    )
    parser.add_argument(
        "--size",
        type=int,
        default=160,
        help="image width/height in pixels; must be even",
    )
    args = parser.parse_args()

    count = generate(args.output_dir, args.size)
    print(f"Generated {count} images in {os.fspath(args.output_dir)}")


if __name__ == "__main__":
    main()
