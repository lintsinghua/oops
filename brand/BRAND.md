# oops brand

The mark is a wordmark whose two o's are a pair of eyes looking back. v2 replaces the previous logo set. Use only the files in `brand/logo/`.

## README header

`wordmark-light.svg` is ink on transparent, for light backgrounds. `wordmark-dark.svg` is paper-colored ink, for dark backgrounds. GitHub switches them with `prefers-color-scheme`:

```html
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="brand/logo/wordmark-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="brand/logo/wordmark-light.svg">
  <img alt="oops" src="brand/logo/wordmark-light.svg">
</picture>
```

## Files

| Use | File |
| --- | --- |
| Wordmark, light theme | `brand/logo/wordmark-light.svg` |
| Wordmark, dark theme | `brand/logo/wordmark-dark.svg` |
| Eyes only, light theme | `brand/logo/mark-light.svg` |
| Eyes only, dark theme | `brand/logo/mark-dark.svg` |
| Avatar | `brand/logo/icon.svg` |
| Favicon and small sizes | `brand/logo/favicon.svg` |

Raster copies live in `brand/logo/png/`.

- Avatar sizes: `icon-180.png`, `icon-512.png`, `icon-1024.png`, rendered from `icon.svg`.
- Favicon sizes: `favicon-16.png`, `favicon-32.png`, `favicon-48.png`, rendered from `favicon.svg`.
- Wordmark heights: `wordmark-light-160h.png`, `wordmark-dark-160h.png`.
- Eye marks: `mark-light-512.png`, `mark-dark-512.png`.

## Color

`wordmark-light.svg` and `mark-light.svg` are for light backgrounds. `wordmark-dark.svg` and `mark-dark.svg` are for dark backgrounds. The designer is replacing this set with a pure black-and-white version under the same filenames. Do not recolor the pupils independently of the letter, and do not add a box, shadow, or tagline inside the logo files.

The social preview is produced by the designer. This repo does not generate `social-preview.png` or `social-preview.svg`.

## Checking the set

```sh
python3 brand/scripts/build_logo.py
sh brand/scripts/export_png.sh
```

`build_logo.py` checks that the v2 filenames are present. `export_png.sh` rasterizes those SVGs into `brand/logo/png/`.
