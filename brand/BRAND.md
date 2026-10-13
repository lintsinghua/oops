# oops — brand guide

> Ctrl+Z for coding agents. · Agent 闯祸，一键撤回。

oops is a safety net. The brand should feel like one: calm, precise, quiet until needed. The reference points are Linear, Vercel, Raycast and Astral (uv / ruff) — one accent colour, lots of space, type doing the work. Nothing playful, no mascots, no exclamation marks. The name already says "oops"; everything else should say "it's fine".

## 1. The mark

<p>
  <img src="logo/oops-mark.svg" height="64" alt="">&nbsp;&nbsp;
  <img src="logo/png/oops-mark-dark-256.png" height="64" alt="" style="background:#0A0A0A">
</p>

**Idea.** Three things in one shape:

1. **A lowercase "o"** — the first letter of the name, and the round shape of the word itself.
2. **An undo arrow** — the ring is drawn 270° clockwise from 12 o'clock; the arrowhead at the top points *counter-clockwise*, i.e. back in time. It's the ↺ you already know from every editor, so it needs no explanation.
3. **A dot in the centre** — the snapshot you return to. It's the only coloured element, the way a single accent marks the selected turn in the TUI (`▸`).

The open quarter on the upper left keeps the ring from reading as a "refresh" icon (those are usually two arrows or a near-closed loop) and gives the mark a direction of travel.

**Geometry** (64 × 64 grid, all integers or halves — the SVG is hand-written, see `scripts/build_logo.py`):

| Element | Value |
|---|---|
| Ring | centre (32, 34.5), r 20, stroke 7, round cap on the tail |
| Arc | from 12 o'clock, 270° clockwise to 9 o'clock |
| Arrowhead | filled triangle, base on the ring's start, half-height = stroke/2 + 5, length = 1.35 × half-height |
| Dot | r 5.5, accent colour |
| Optical centring | ring centre sits 2.5 below the grid centre so the arrowhead and the ring bottom have equal 6-unit margins |

**Favicon cut** (`oops-favicon*.svg`): for 16–48 px the stroke goes to 9, the ring to r 19 and the dot to r 6.5, so the ring and dot don't blur together.

## 2. Files

All sources are SVG, text is converted to outlines (renders identically on GitHub, in browsers and in `rsvg-convert`). Regenerate everything with `./scripts/export_png.sh` (needs `rsvg-convert` and Python `fontTools`).

| File | Use |
|---|---|
| `logo/oops-mark.svg` | Mark for light backgrounds (ink `#0A0A0A`, dot `#B76E00`) |
| `logo/oops-mark-dark.svg` | Mark for dark backgrounds (ink `#EDEDED`, dot `#F5A524`) |
| `logo/oops-mark-mono.svg` | Single colour, uses `currentColor` — inherits the surrounding text colour |
| `logo/oops-favicon.svg`, `-dark.svg` | Small-size cut for 16–48 px |
| `logo/oops-wordmark.svg`, `-dark.svg` | Horizontal lockup: mark + "oops" (Geist SemiBold, outlined) |
| `logo/oops-icon.svg` | App icon / avatar: dark mark on a `#0A0A0A` rounded square (radius 22%) |
| `logo/png/` | PNG exports: mark 64–1024, favicon 16/32/48, wordmark 76/152/304 px tall, icon 180/512/1024 |
| `social-preview.svg` / `.png` | 1280 × 640 GitHub social preview / Open Graph image |

In a README, pair the light and dark versions with `<picture>`:

```html
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="brand/logo/oops-wordmark-dark.svg">
  <img alt="oops" src="brand/logo/oops-wordmark.svg" height="64">
</picture>
```

## 3. Colour

One accent, amber — the colour of a caution light that isn't an alarm. Everything else is neutral. The values match the CLI palette in DESIGN.md §5.1, so the logo, the README and the terminal look like one product.

| Role | Dark background | Light background | Notes |
|---|---|---|---|
| **Accent (amber)** | `#F5A524` | `#B76E00` | The dot, key commands, selection. Never for large fills or body text |
| Ink | `#EDEDED` | `#0A0A0A` | Mark, wordmark, headings |
| Background | `#0A0A0A` | `#FFFFFF` | Social cards, icon tile |
| Muted | `#8B949E` | `#57606A` | Secondary text, paths, timestamps |
| Surface / line | `#111315` / `#262A30` | `#F6F8FA` / `#D0D7DE` | Cards, terminal frames, dividers |

Semantic colours exist only inside terminal output and screenshots of it — never in the logo:

| | Dark | Light | Meaning |
|---|---|---|---|
| success | `#3FB950` | `#1A7F37` | `✓`, restore, `+` lines |
| danger | `#F85149` | `#CF222E` | `✗`, remove, `−` lines |
| warning | `#D29922` | `#9A6700` | `!`, kept, conflicts |
| info | `#58A6FF` | `#0969DA` | revert, modified |

Rule of thumb: in any one composition, amber covers less than 5% of the area.

## 4. Typography

| Use | Typeface | Fallback |
|---|---|---|
| Wordmark, headings, UI | **Geist** (SemiBold 600 for the wordmark and headlines, Regular 400 for text) | Inter, system-ui |
| Code, terminal, commands | **Geist Mono** | JetBrains Mono, ui-monospace |
| Chinese | System CJK sans: PingFang SC / Noto Sans SC / Microsoft YaHei | — |

Both Geist faces are open source (SIL OFL 1.1). Headlines use slight negative tracking (−0.02 to −0.025 em). No italics, no all-caps headings, no more than two weights on a page.

## 5. Name and voice

- Always lowercase: **oops** — also at the start of a sentence. Never "Oops", "OOPS" or "Oops!".
- When you mean the command, set it in code: `oops`, `oops undo`.
- The brand name does the joking; the copy doesn't. Short, active, present tense. Result first, details after (DESIGN.md §5.2).
- No emoji, no banners, no ASCII art — in the CLI and in brand materials.
- Be factual about other tools. Cite the docs or the issue; never "X is broken". oops works *alongside* built-in checkpoints.
- Taglines: **"Ctrl+Z for coding agents."** (primary) · **"Agent 闯祸，一键撤回。"** (Chinese) · "Undo anything your agent did. Even `rm -rf`." (secondary).

## 6. Usage rules

**Clear space.** Keep at least the dot's diameter (11/64 of the mark's height) free on every side of the mark; for the wordmark, at least half the mark's height.

**Minimum size.** Mark: 16 px (use the favicon cut below 48 px). Wordmark: 20 px tall / 64 px wide; below that, use the mark alone.

**Backgrounds.** Light version on white or light grey; dark version on `#0A0A0A` or anything darker than `#333`. On photos or colour, use `oops-mark-mono.svg` in pure white or black.

**Do**
- Use the files as they are.
- Use the mark alone where the name is already present (avatars, favicons, the CLI's own docs).
- Let the mono version inherit colour in docs and badges.

**Don't**
- Recolour the ring amber, or make the dot any colour other than the accent (or ink, in mono).
- Rotate or mirror the mark — the arrow must point counter-clockwise.
- Close the ring, add a second arrow, add shadows, gradients, outlines or 3D.
- Retype the wordmark in another font, or change the spacing between mark and text.
- Put the mark inside another shape, except the official `oops-icon.svg` tile.
- Use the light version on dark backgrounds or vice versa.

## 7. Social preview

`social-preview.png` (1280 × 640): wordmark, the tagline, one sentence, and the real `oops undo` output from DESIGN.md §1.4 in a quiet terminal card. Keep the 40 px safe margin GitHub crops to. If the CLI copy changes, update the card text in `scripts/build_social.py` and re-export.
