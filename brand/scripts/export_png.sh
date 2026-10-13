#!/usr/bin/env bash
# Export brand PNGs from the SVG sources. Needs rsvg-convert (librsvg2-bin / brew install librsvg).
set -euo pipefail
cd "$(dirname "$0")/.."
python3 scripts/build_logo.py
mkdir -p logo/png
for v in "" "-dark"; do
  for s in 64 256 512 1024; do rsvg-convert -w $s -h $s logo/oops-mark$v.svg -o logo/png/oops-mark$v-$s.png; done
  for s in 16 32 48; do rsvg-convert -w $s -h $s logo/oops-favicon$v.svg -o logo/png/oops-favicon$v-$s.png; done
  for h in 76 152 304; do rsvg-convert -h $h logo/oops-wordmark$v.svg -o logo/png/oops-wordmark$v-${h}h.png; done
done
for s in 180 512 1024; do rsvg-convert -w $s -h $s logo/oops-icon.svg -o logo/png/oops-icon-$s.png; done
python3 scripts/build_social.py
rsvg-convert -w 1280 -h 640 social-preview.svg -o social-preview.png
echo "done"
