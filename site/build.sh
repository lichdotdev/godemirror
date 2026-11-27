#!/bin/sh
# Builds the site into a directory, _site by default: the pages, the media
# from docs/media and the wasm demo, built without chroma and gzipped.
# Run from anywhere: site/build.sh [out]
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-$root/_site}
rm -rf "$out"
mkdir -p "$out/media"
cp "$root"/site/index.html "$root"/site/demo.js "$root"/site/demo-frame.html "$out"/
for f in hero.png diagnostics.png code-actions.png rename.png tour.webp social-preview.png; do
	cp "$root/docs/media/$f" "$out/media/"
done
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out"/
cd "$root"
GOOS=js GOARCH=wasm go build -tags godemirror_nochroma -trimpath -ldflags="-s -w" -o "$out/main.wasm" ./cmd/wasmdemo
gzip -9 -n "$out/main.wasm"
# Named by its content, so a browser never runs a cached build of another.
name=main.$(sha256sum "$out/main.wasm.gz" | cut -c1-12).wasm.gz
mv "$out/main.wasm.gz" "$out/$name"
sed -i "s/\"main\.wasm\.gz\"/\"$name\"/" "$out/demo-frame.html"
grep -q "$name" "$out/demo-frame.html"
ls -l "$out/$name"
