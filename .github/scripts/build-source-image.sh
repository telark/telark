#!/usr/bin/env bash
# Builds <source-image>: the corresponding source of every GPL, LGPL or MPL Alpine package in
# <image>, one `abuild srcpkg` bundle (APKBUILD, patches, upstream archives) per package, taken
# from the aports commit the package was built from. --push publishes it.
# usage: build-source-image.sh <image> <source-image> [--push]
set -euo pipefail

image=$1
source_image=$2
push=${3:-}
work=$(mktemp -d)
ctr=$(docker create "$image")
trap 'docker rm "$ctr" >/dev/null; rm -rf "$work"' EXIT

# The Go images are FROM scratch: no apk database, nothing to publish.
if ! docker cp "$ctr:/lib/apk/db/installed" "$work/installed" 2>/dev/null; then
  echo "No Alpine packages in $image, no source image needed."
  exit 0
fi
docker cp "$ctr:/etc/alpine-release" "$work/alpine-release"
branch=$(cut -d. -f1,2 "$work/alpine-release")

# One line per origin: origin, version, aports commit, license.
awk -F: '
  function flush() {
    if (origin != "" && license ~ /GPL|MPL/) print origin, version, commit, license
    origin = license = ""
  }
  /^o:/ { origin = $2 }
  /^V:/ { version = $2 }
  /^c:/ { commit = $2 }
  /^L:/ { license = substr($0, 3) }
  /^$/  { flush() }
  END   { flush() }
' "$work/installed" | sort -u -k1,1 > "$work/packages"

mkdir -p "$work/src"
docker run --rm -e BRANCH="$branch" -v "$work:/w" "alpine:$branch" sh -euc '
  apk add -q abuild git
  # The GitHub mirror of aports: gitlab.alpinelinux.org answers GitHub runners with 418.
  git init -q /tmp/aports && git -C /tmp/aports remote add origin https://github.com/alpinelinux/aports.git
  while read -r origin version commit license; do
    git -C /tmp/aports fetch -q --depth=1 --filter=blob:none origin "$commit"
    # srcpkg names the top folder of the bundle after this directory, as in aports.
    git -C /tmp/aports archive --prefix="$origin/" "$commit:main/$origin" | tar -x -C /tmp
    cd "/tmp/$origin"
    SRCDEST=/tmp/distfiles REPODEST=/w DISTFILES_MIRROR="https://distfiles.alpinelinux.org/distfiles/v$BRANCH" \
      abuild -F -q fetch verify srcpkg
  done < /w/packages
'
{ echo "# origin version aports-commit license"; cat "$work/packages"; } > "$work/src/index.txt"

printf 'FROM scratch\nCOPY src/ /\n' > "$work/Dockerfile"
docker build -q -t "$source_image" \
  --label org.opencontainers.image.description="Corresponding source of the GPL, LGPL and MPL Alpine packages in $image" \
  --label org.opencontainers.image.source=https://github.com/telark/telark \
  "$work"
ls -l "$work/src"
if [[ "$push" == --push ]]; then docker push "$source_image"; fi
