# oops — brand guide

> Ctrl+Z for Agent: precise undo & redo in one step. · Agent 的 Ctrl+Z：一键精准撤销/重做

oops is a safety net. The brand should feel like one: calm, precise, quiet until needed. The logo has one moment of character, the two o's glancing back. Everything around it stays quiet: pure black and white, lots of space, type doing the work. No mascots, no exclamation marks. The name and the eyes already say "oops"; everything else should say "it's fine".

## 1. The logo

<p>
  <img src="logo/wordmark-light.svg" height="64" alt="oops">&nbsp;&nbsp;
  <img src="logo/png/wordmark-dark-160h.png" height="64" alt="oops" style="background:#000000">
</p>

**Idea: glance back.** The two o's of "oops" are a pair of eyes looking back over the shoulder, up and to the left. It is the "oops" moment, and it is also what the tool does: look back at what the agent just did. The rest of the word is plain type, so the eyes are the only detail.

**Construction** (all text is outlined, so it renders identically everywhere; the SVGs in `logo/` are the files in this repo):

| Element | Value |
|---|---|
| Typeface | Fredoka, weight 500, width 100 (SIL OFL 1.1), outlined with fontTools |
| Spacing | Gaps between letter ink boxes: o–o 28, o–p 38, p–s 28 (font units, 1000 UPM). Tighter than the font default, so the two eyes read as a pair |
| Pupils | One solid disc in each o's counter: radius 0.58 × counter radius, centre offset (−0.40, −0.12) × counter radius, i.e. the pupil touches the left side of the counter and looks back and slightly up |
| Colour | One colour only: `#000000` on light, `#FFFFFF` on dark. The pupils are the same colour as the letters |
| Mark | The two o's alone ("the eyes"), same geometry as in the wordmark |
| App icon | The eyes, 48/64 wide, centred on a 64 × 64 rounded square (radius 22.5%). `icon.svg`: black tile with white eyes; `icon-dark.svg`: white tile with black eyes |
| Favicon cut | For 16–48 px a single eye: ring r 22, counter r 12.5, pupil r 8 at (27, 30.5) on the 64 grid. Two eyes blur into one shape at 16 px, a single one doesn't |

## 2. Files

All logo files are in `logo/`. The ring-and-arrow set is not in this repo, and the generator that drew these SVGs was not included.

| File | Use |
|---|---|
| `wordmark-light.svg` / `wordmark-dark.svg` | Primary logo, black for light backgrounds / white for dark backgrounds |
| `wordmark-mono.svg` | Uses `currentColor`, so it inherits the surrounding text colour |
| `mark-light.svg` / `mark-dark.svg` / `mark-mono.svg` | The eyes alone, where the name is already present |
| `icon.svg` | App icon / avatar: white eyes on a black tile. Default, and the one to use on light pages |
| `icon-dark.svg` | Black eyes on a white tile, for dark pages where a black tile would disappear |
| `favicon.svg` / `favicon-dark.svg` | Single-eye cut for 16–48 px, black tile / white tile |
| `png/` | Wordmark 76/160/304 px tall; mark 64/512/1024; icon 180/512/1024; favicon 16/32/48 (each in both versions) |
| `board-bw.png` | Overview of every version on black and white, at large and small sizes |
| `social-preview.svg`, `social-preview.png` | 1280 × 640 GitHub social preview, next to this file (§7) |

In a README, pair the light and dark versions with `<picture>`:

```html
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="brand/logo/wordmark-dark.svg">
  <img alt="oops" src="brand/logo/wordmark-light.svg" height="64">
</picture>
```

## 3. Colour

The brand is pure black and white. There is no brand accent colour: no amber, no gradients, no tints in the logo, the icon, the social card or any other brand material.

| Role | On dark | On light | Notes |
|---|---|---|---|
| Ink | `#FFFFFF` | `#000000` | Logo, headings, body text on brand surfaces |
| Background | `#000000` | `#FFFFFF` | Icon tiles, social card, brand pages |
| Muted | `#8B8B8B` | `#6B6B6B` | Secondary text, URLs, captions |
| Surface / line | `#0D0D0D` / `#2A2A2A` | `#F5F5F5` / `#E2E2E2` | Cards, terminal frames, dividers |

Colour only exists inside the terminal: the CLI palette in DESIGN.md §5.1 (accent, success, danger, warning, info) applies to real CLI output and the TUI, which follow the user's terminal theme. It is not part of the brand. When brand material shows terminal output (social card, slides, the README header), render it in the monochrome palette above and tell actions apart by their words (`restored`, `removed`, `reverted`) and by weight, not colour. Screenshots and GIFs of the real CLI stay in colour.

## 4. Typography

| Use | Typeface | Fallback |
|---|---|---|
| Logo | **Fredoka** Medium (500), outlined: never typed live, never retyped | — |
| Headings, UI, body | **Geist** (SemiBold 600 for headlines, Regular 400 for text) | Inter, system-ui |
| Code, terminal, commands | **Geist Mono** | JetBrains Mono, ui-monospace |
| Chinese | System CJK sans: PingFang SC / Noto Sans SC / Microsoft YaHei | — |

All faces are open source (SIL OFL 1.1). Fredoka is used only in the logo; don't set headlines in it. Headlines use slight negative tracking (−0.02 to −0.025 em). No italics, no all-caps headings, no more than two weights on a page.

## 5. Name and voice

- Always lowercase: **oops** — also at the start of a sentence. Never "Oops", "OOPS" or "Oops!".
- When you mean the command, set it in code: `oops`, `oops undo`.
- The brand name does the joking; the copy doesn't. Short, active, present tense. Result first, details after (DESIGN.md §5.2).
- No emoji, no banners, no ASCII art — in the CLI and in brand materials.
- Be factual about other tools. Cite the docs or the issue; never "X is broken". oops works *alongside* built-in checkpoints.
- Taglines: **"Ctrl+Z for Agent: precise undo & redo in one step."** (primary) · **"Agent 的 Ctrl+Z：一键精准撤销/重做"** (Chinese).

## 6. Usage rules

**Clear space.** Keep at least the height of one o free on every side of the wordmark, and half the mark's height around the mark.

**Minimum size.** Wordmark: 20 px tall / 56 px wide (below that the pupils merge with the letters); below that, use the favicon. Mark (two eyes): 32 px wide. Below 48 px use `favicon.svg` / `favicon-dark.svg`.

**Backgrounds.** Black logo on white or light grey, white logo on black or anything darker than `#333`. On photos or colour, use the mono version in pure white or pure black. Use `icon.svg` (black tile) on light pages and `icon-dark.svg` (white tile) on dark pages.

**Do**
- Use the files as they are.
- Use the mark or the icon alone where the name is already present (avatars, favicons, the CLI's own docs).
- Let the mono version inherit colour in docs and badges.

**Don't**
- Add any colour, gradient, tint, shadow, outline or 3D effect, or colour the pupils differently from the letters.
- Move the pupils or change where they look. They always look back, to the left.
- Add lashes, a mouth, blush or any other face details. The two eyes are enough.
- Retype the wordmark (live text, another font or weight), change the letter spacing, or slant or bounce the letters.
- Put the logo inside another shape, except the official icon tiles.
- Use the light version on dark backgrounds or vice versa.

## 7. Social preview

`social-preview.png` (1280 × 640) and `social-preview.svg` are the designer export: black and white, white wordmark, the tagline **Ctrl+Z for Agent: precise undo & redo in one step.**, the line **Precise undo & redo in one step, right inside Claude Code, Codex, Gemini CLI and Cursor CLI.**, and a monochrome terminal card. Action words are set in bold instead of colour. Keep the 40 px safe margin GitHub crops to. Do not regenerate these files from an older script. The card text follows DESIGN.md §1.4 and is sample copy, not a recording of this build.
