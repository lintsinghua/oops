#!/usr/bin/env python3
"""Build social-preview.svg (1280×640, GitHub social preview / OG image).
Uses Geist + Geist Mono via fontconfig; render with rsvg-convert to PNG."""
import os, re
from xml.sax.saxutils import escape
ROOT = os.path.join(os.path.dirname(__file__), "..")
wm = open(os.path.join(ROOT, "logo", "oops-wordmark-dark.svg")).read()
inner = re.search(r"</title>(.*)</svg>", wm, re.S).group(1)
wm_w, wm_h = map(float, re.search(r'viewBox="0 0 ([\d.]+) ([\d.]+)"', wm).groups())

BG, FG, MUTED, AMBER, GREEN, RED, BLUE, LINE = "#0A0A0A", "#EDEDED", "#8B949E", "#F5A524", "#3FB950", "#F85149", "#58A6FF", "#262A30"
SANS, MONO = "Geist, Inter, sans-serif", "'Geist Mono', 'JetBrains Mono', monospace"

def t(x, y, s, size, fill, family=SANS, weight=400, extra=""):
    return f'<text x="{x}" y="{y}" font-family="{family}" font-size="{size}" font-weight="{weight}" fill="{fill}" {extra}>{s}</text>'

def span(s, fill, weight=None):
    w = f' font-weight="{weight}"' if weight else ""
    return f'<tspan fill="{fill}"{w}>{escape(s)}</tspan>'

scale = 52 / wm_h
parts = [f'<rect width="1280" height="640" fill="{BG}"/>',
         f'<g transform="translate(80 72) scale({scale:.4f})">{inner}</g>',
         t(1200, 108, "github.com/lintsinghua/oops", 20, MUTED, MONO, extra='text-anchor="end"'),
         t(80, 236, "Ctrl+Z for coding agents.", 64, FG, SANS, 600, 'letter-spacing="-1.6"'),
         t(80, 286, escape("Undo anything Claude Code, Codex, Gemini or Cursor did to your files. Even rm -rf."), 25, MUTED)]

# terminal card — copy follows DESIGN.md §1.4 verbatim
cx, cy, cw, ch = 80, 336, 1120, 236
parts.append(f'<rect x="{cx}" y="{cy}" width="{cw}" height="{ch}" rx="14" fill="#111315" stroke="{LINE}"/>')
lines = [
    span("$ ", MUTED) + span("oops undo", FG),
    span("✓ ", GREEN) + span("Undid turn #42 · claude · ", FG) + span('"clean up the build scripts"', MUTED),
    "",
    span("  restored  ", GREEN) + span("39 files   ", FG) + span("src/ (38), .env", MUTED),
    span("  removed   ", RED) + span(" 1 file    ", FG) + span("scripts/build.mjs", MUTED),
    span("  reverted  ", BLUE) + span(" 1 file    ", FG) + span("package.json", MUTED),
]
y = cy + 46
for ln in lines:
    if ln:
        parts.append(f'<text x="{cx + 36}" y="{y}" font-family="{MONO}" font-size="20" xml:space="preserve">{ln}</text>')
    y += 30
parts.append(t(cx + cw - 36, cy + ch - 30, span("Changed your mind? ", MUTED) + span("oops redo", AMBER, 600), 20, FG, MONO, extra='text-anchor="end" xml:space="preserve"'))

svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="640" viewBox="0 0 1280 640">'
       + "".join(parts) + "</svg>\n")
open(os.path.join(ROOT, "social-preview.svg"), "w").write(svg)
