#!/bin/sh
# Rasterize the v2 logo SVGs into brand/logo/png.
# Favicon sizes come from favicon.svg. Avatar sizes come from icon.svg.
set -eu
cd "$(dirname "$0")/.."
mkdir -p logo/png

rsvg-convert -h 160 logo/wordmark-light.svg -o logo/png/wordmark-light-160h.png
rsvg-convert -h 160 logo/wordmark-dark.svg -o logo/png/wordmark-dark-160h.png
rsvg-convert -w 512 logo/mark-light.svg -o logo/png/mark-light-512.png
rsvg-convert -w 512 logo/mark-dark.svg -o logo/png/mark-dark-512.png
rsvg-convert -w 180 logo/icon.svg -o logo/png/icon-180.png
rsvg-convert -w 512 logo/icon.svg -o logo/png/icon-512.png
rsvg-convert -w 1024 logo/icon.svg -o logo/png/icon-1024.png
rsvg-convert -w 16 logo/favicon.svg -o logo/png/favicon-16.png
rsvg-convert -w 32 logo/favicon.svg -o logo/png/favicon-32.png
rsvg-convert -w 48 logo/favicon.svg -o logo/png/favicon-48.png

echo "exported brand/logo/png"
