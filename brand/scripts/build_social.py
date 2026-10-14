#!/usr/bin/env python3
"""Build social-preview.svg (1280×640, GitHub social preview / OG image), monochrome.
Logo: logo-v2/A-glance wordmark (outlined). Text uses Geist + Geist Mono via fontconfig; render with rsvg-convert."""
import os, re
from xml.sax.saxutils import escape
ROOT = os.path.join(os.path.dirname(__file__), "..")
wm = open(os.path.join(ROOT, "logo-v2", "A-glance", "wordmark-dark.svg")).read()
inner = re.search(r"</title>(.*)</svg>", wm, re.S).group(1)
vx, vy, wm_w, wm_h = map(float, re.search(r'viewBox="([-\d.]+) ([-\d.]+) ([\d.]+) ([\d.]+)"', wm).groups())

# pure black & white; the terminal card distinguishes actions by word, not colour
BG, FG, MUTED, LINE, CARD = "#000000", "#FFFFFF", "#8B8B8B", "#2A2A2A", "#0D0D0D"
SANS, MONO = "Geist, Inter, sans-serif", "'Geist Mono', 'JetBrains Mono', monospace"

def t(x, y, s, size, fill, family=SANS, weight=400, extra=""):
    return f'<text x="{x}" y="{y}" font-family="{family}" font-size="{size}" font-weight="{weight}" fill="{fill}" {extra}>{s}</text>'

def span(s, fill, weight=None):
    w = f' font-weight="{weight}"' if weight else ""
    return f'<tspan fill="{fill}"{w}>{escape(s)}</tspan>'

# the wordmark SVG carries 22% padding top and bottom; size so the ink is ~60 px tall
scale = 60 * 1.44 / wm_h
pad = 0.22 * wm_h / 1.44
parts = [f'<rect width="1280" height="640" fill="{BG}"/>',
         f'<g transform="translate({80 - (vx + pad) * scale:.2f} {82 - (vy + pad) * scale:.2f}) scale({scale:.4f})">{inner}</g>',  # ink box at (80, 82)
         t(1200, 108, "github.com/lintsinghua/oops", 20, MUTED, MONO, extra='text-anchor="end"'),
         t(80, 236, "Ctrl+Z for Agent.", 64, FG, SANS, 600, 'letter-spacing="-1.6"'),
         t(80, 286, escape("Precise undo & redo in one step, right inside Claude Code, Codex, Gemini CLI and Cursor CLI."), 25, MUTED)]

# terminal card — copy follows DESIGN.md §1.4 verbatim
cx, cy, cw, ch = 80, 336, 1120, 236
parts.append(f'<rect x="{cx}" y="{cy}" width="{cw}" height="{ch}" rx="14" fill="{CARD}" stroke="{LINE}"/>')
lines = [
    span("$ ", MUTED) + span("oops undo", FG),
    span("✓ ", FG) + span("Undid turn #42 · claude · ", FG) + span('"clean up the build scripts"', MUTED),
    "",
    span("  restored  ", FG, 600) + span("39 files   ", FG) + span("src/ (38), .env", MUTED),
    span("  removed   ", FG, 600) + span(" 1 file    ", FG) + span("scripts/build.mjs", MUTED),
    span("  reverted  ", FG, 600) + span(" 1 file    ", FG) + span("package.json", MUTED),
]
y = cy + 46
for ln in lines:
    if ln:
        parts.append(f'<text x="{cx + 36}" y="{y}" font-family="{MONO}" font-size="20" xml:space="preserve">{ln}</text>')
    y += 30
parts.append(t(cx + cw - 36, cy + ch - 30, span("Changed your mind? ", MUTED) + span("oops redo", FG, 600), 20, FG, MONO, extra='text-anchor="end" xml:space="preserve"'))

svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="640" viewBox="0 0 1280 640">'
       + "".join(parts) + "</svg>\n")
open(os.path.join(ROOT, "social-preview.svg"), "w").write(svg)
