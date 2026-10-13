#!/usr/bin/env python3
"""Generate oops brand SVGs. Text is converted to outlines (Geist SemiBold, OFL)
so the files render identically everywhere (GitHub, browsers, rsvg)."""
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen
import os

OUT = os.path.join(os.path.dirname(__file__), "..", "logo")
GEIST = "/usr/share/fonts/truetype/sand-box/google/Geist/Geist-VariableFont_wght.ttf"

INK_LIGHT, INK_DARK = "#0A0A0A", "#EDEDED"     # mark/text on light bg, on dark bg
AMBER_DARK, AMBER_LIGHT = "#F5A524", "#B76E00"  # DESIGN §5.1 accent

# ---- the mark (64×64 grid) ------------------------------------------------
# A ring (the "o") drawn 270° clockwise from 12 o'clock to 9 o'clock; an arrowhead
# at 12 o'clock points counter-clockwise (= back in time). The dot in the centre
# is the snapshot you return to.
# Optical centring: the arrowhead reaches higher than the ring, so the ring
# centre sits at y=34.5 to leave equal 6-unit margins top and bottom.
def mark(ink, accent, tx=0, ty=0, s=1, small=False):
    sw, r, dot = (9, 19, 6.5) if small else (7, 20, 5.5)   # small = favicon-tuned
    cy = 34.5
    top = cy - r
    ah = sw / 2 + 5                                         # arrowhead half-height
    return (f'<g transform="translate({tx} {ty}) scale({s})">'
            f'<path d="M32 {top}A{r} {r} 0 1 1 {32 - r} {cy}" fill="none" stroke="{ink}" stroke-width="{sw}" stroke-linecap="round"/>'
            f'<path d="M33 {top - ah}L{33 - ah * 1.35:.2f} {top}L33 {top + ah}z" fill="{ink}"/>'
            f'<circle cx="32" cy="{cy}" r="{dot}" fill="{accent}"/></g>')

def svg(w, h, body, title="oops"):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
            f'role="img" aria-label="{title}"><title>{title}</title>{body}</svg>\n')

def write(name, content):
    with open(os.path.join(OUT, name), "w") as fh:
        fh.write(content)

# ---- wordmark text to outlines --------------------------------------------
font = instancer.instantiateVariableFont(TTFont(GEIST), {"wght": 600})
gs, cmap = font.getGlyphSet(), font.getBestCmap()
UPM = font["head"].unitsPerEm

def text_path(txt, size, x0, baseline, tracking=-0.02):
    pen = SVGPathPen(gs)
    scale = size / UPM
    x = x0
    for ch in txt:
        g = cmap[ord(ch)]
        gs[g].draw(TransformPen(pen, (scale, 0, 0, -scale, x, baseline)))
        x += gs[g].width * scale + tracking * size
    return pen.getCommands(), x - tracking * size

MS = 1.125                      # mark scale inside the wordmark (72 px tall)
SIZE = 68                       # x-height 0.53em ≈ 36
BASE = 34.5 * MS + 18           # x-height band centred on the ring centre
d, xend = text_path("oops", SIZE, 64 * MS + 14, BASE)
W, H = int(round(xend + 2)), 76

for suffix, ink, acc in (("", INK_LIGHT, AMBER_LIGHT), ("-dark", INK_DARK, AMBER_DARK)):
    write(f"oops-mark{suffix}.svg", svg(64, 64, mark(ink, acc)))
    write(f"oops-wordmark{suffix}.svg", svg(W, H, mark(ink, acc, 0, 0, MS) + f'<path d="{d}" fill="{ink}"/>'))
    write(f"oops-favicon{suffix}.svg", svg(64, 64, mark(ink, acc, small=True)))

# single-colour mark that follows the surrounding text colour
write("oops-mark-mono.svg", svg(64, 64, mark("currentColor", "currentColor")))
# app icon / avatar: mark on a rounded square
write("oops-icon.svg", svg(512, 512,
      '<rect width="512" height="512" rx="112" fill="#0A0A0A"/>' + mark(INK_DARK, AMBER_DARK, 96, 96, 5)))
print("wordmark size", W, H)
