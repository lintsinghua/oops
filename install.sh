#!/bin/sh
# Install oops without sudo. macOS and Linux.
# Override with OOPS_VERSION, OOPS_INSTALL_DIR, and OOPS_DOWNLOAD_BASE (tests).
set -eu

die() {
	printf '%s\n' "$1" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || die "curl is required"
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
	die "sha256sum or shasum is required"
fi
[ -n "${HOME:-}" ] || die "HOME is not set"

download_base=${OOPS_DOWNLOAD_BASE:-https://github.com/lintsinghua/oops/releases/download}
download_base=${download_base%/}
install_dir=${OOPS_INSTALL_DIR:-"$HOME/.local/bin"}
while [ "$install_dir" != "/" ] && [ "$install_dir" != "" ]; do
	case "$install_dir" in
	*/) install_dir=${install_dir%/} ;;
	*) break ;;
	esac
done
[ -n "$install_dir" ] || die "OOPS_INSTALL_DIR is empty"

uname_s=$(uname -s)
uname_m=$(uname -m)
case "$uname_s" in
Linux) goos=linux ;;
Darwin) goos=darwin ;;
MINGW* | MSYS* | CYGWIN*)
	die "On Windows, run: irm https://raw.githubusercontent.com/lintsinghua/oops/main/install.ps1 | iex"
	;;
*) die "unsupported OS: $uname_s" ;;
esac
case "$uname_m" in
x86_64 | amd64) goarch=amd64 ;;
aarch64 | arm64) goarch=arm64 ;;
*) die "unsupported architecture: $uname_m" ;;
esac

hash_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

read_version() {
	bin=$1
	out=$("$bin" version 2>/dev/null) || true
	[ -n "$out" ] || return 1
	esc=$(printf '\033')
	printf '%s\n' "$out" | sed "s/${esc}\\[[0-9;]*[A-Za-z]//g" | awk 'NR==1 { print $2; exit }'
}

tmpdir=$(mktemp -d)
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT INT TERM

version=${OOPS_VERSION:-}
if [ -z "$version" ]; then
	api="https://api.github.com/repos/lintsinghua/oops/releases/latest"
	curl -fsSL -A oops-install "$api" -o "$tmpdir/release.json" || die "could not read the latest release; set OOPS_VERSION"
	version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmpdir/release.json" | head -n 1)
	[ -n "$version" ] || die "latest release has no tag_name; set OOPS_VERSION"
fi
case "$version" in
v*) version=${version#v} ;;
esac

asset="oops_${version}_${goos}_${goarch}.tar.gz"
url="${download_base}/v${version}/${asset}"
sum_url="${download_base}/v${version}/checksums.txt"
printf 'Downloading oops %s (%s/%s)\n' "$version" "$goos" "$goarch" >&2

previous=
if [ -e "${install_dir}/oops" ]; then
	previous=$(read_version "${install_dir}/oops" || true)
	[ -n "$previous" ] || previous=unknown
fi

archive="$tmpdir/$asset"
curl -fsSL "$url" -o "$archive" || die "download failed: $url"
curl -fsSL "$sum_url" -o "$tmpdir/checksums.txt" || die "download failed: $sum_url"

expected=$(awk -v name="$asset" '
	{
		hash = $1
		file = $2
		gsub(/\r/, "", hash)
		gsub(/\r/, "", file)
		sub(/^\*/, "", file)
		if (file == name) {
			print hash
			exit
		}
	}
' "$tmpdir/checksums.txt")
expected=$(printf '%s' "$expected" | tr 'A-F' 'a-f')
raw_hash=$(hash_file "$archive") || die "sha256 failed for ${asset}"
actual=$(printf '%s' "$raw_hash" | tr 'A-F' 'a-f')
[ -n "$expected" ] || die "checksums.txt has no entry for ${asset}"
if [ "$actual" != "$expected" ]; then
	die "checksum mismatch for ${asset}"
fi

extract="$tmpdir/extract"
mkdir -p "$extract"
tar -xzf "$archive" -C "$extract"
find "$extract" -type f -name oops > "$tmpdir/bins"
src=
while IFS= read -r candidate; do
	src=$candidate
	break
done < "$tmpdir/bins"
[ -n "$src" ] || die "archive does not contain oops"

mkdir -p "$install_dir"
staged="${install_dir}/oops.new.$$"
cp "$src" "$staged"
chmod 755 "$staged"
mv -f "$staged" "${install_dir}/oops"

installed=$(read_version "${install_dir}/oops" || true)
[ -n "$installed" ] || die "installed binary did not print a version"

if [ -n "$previous" ]; then
	printf 'upgraded from %s to %s\n' "$previous" "$installed"
else
	printf 'installed oops %s\n' "$installed"
fi

on_path=0
saved_ifs=$IFS
IFS=:
set -f
for entry in $PATH; do
	while [ "$entry" != "/" ] && [ "$entry" != "" ]; do
		case "$entry" in
		*/) entry=${entry%/} ;;
		*) break ;;
		esac
	done
	if [ "$entry" = "$install_dir" ]; then
		on_path=1
		break
	fi
done
set +f
IFS=$saved_ifs
if [ "$on_path" -eq 0 ]; then
	escaped=$(printf '%s' "$install_dir" | sed 's/\\/\\\\/g; s/"/\\"/g; s/`/\\`/g; s/\$/\\$/g')
	printf 'export PATH="%s:$PATH"\n' "$escaped"
fi

printf '%s\n' "oops init"
