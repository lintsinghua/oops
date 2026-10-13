#!/usr/bin/env python3
"""Check the designer-authored oops logo set.

The letterforms live in brand/logo and are not redrawn here. v2 replaces v1:
the two o's are a pair of eyes looking back, and the file names changed.
This script only checks that the v2 files are present and internally consistent.
"""

from __future__ import annotations

import sys
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LOGO = ROOT / "logo"

# Canonical v2 names. Do not reintroduce v1 filenames.
SVG = (
    "wordmark-light.svg",
    "wordmark-dark.svg",
    "mark-light.svg",
    "mark-dark.svg",
    "icon.svg",
    "favicon.svg",
)
PNG = (
    "png/wordmark-light-160h.png",
    "png/wordmark-dark-160h.png",
    "png/mark-light-512.png",
    "png/mark-dark-512.png",
    "png/icon-180.png",
    "png/icon-512.png",
    "png/icon-1024.png",
    "png/favicon-16.png",
    "png/favicon-32.png",
    "png/favicon-48.png",
)

# Light files are black ink; dark files are white ink. Accept #000/#fff and
# the interim ink values until the black-and-white drop-in lands.
LIGHT_FILLS = {"#000", "#000000", "#17120E"}
DARK_FILLS = {"#FFF", "#FFFFFF", "#F5F0EA"}


def local(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def main() -> int:
    missing = [name for name in SVG + PNG if not (LOGO / name).is_file()]
    if missing:
        print("missing logo files:", ", ".join(missing), file=sys.stderr)
        return 1
    for name in ("wordmark-light.svg", "wordmark-dark.svg", "mark-light.svg", "mark-dark.svg"):
        tree = ET.parse(LOGO / name)
        circles = [el for el in tree.getroot().iter() if local(el.tag) == "circle"]
        if len(circles) < 2:
            print(f"{name}: expected two eye pupils", file=sys.stderr)
            return 1
        allowed = LIGHT_FILLS if "light" in name else DARK_FILLS
        fills = {el.get("fill", "").upper() for el in circles}
        if not fills or not fills <= allowed:
            print(f"{name}: pupil fill {fills}, want black or white ink", file=sys.stderr)
            return 1
        # Pupils sit left of each counter, so the pair looks back.
        xs = sorted(float(el.get("cx")) for el in circles)
        if xs[1] - xs[0] < 40:
            print(f"{name}: pupils are not a pair", file=sys.stderr)
            return 1
    print(f"logo set ok ({len(SVG)} svg, {len(PNG)} png)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
