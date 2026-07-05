#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "${version}" ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="${root}/dist"
tmp="${dist}/tmp"

rm -rf "${dist}"
mkdir -p "${tmp}"

targets=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

for target in "${targets[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  name="cairnline_${version}_${goos}_${goarch}"
  work="${tmp}/${name}"
  mkdir -p "${work}"

  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${version}" \
    -o "${work}/cairnline" \
    "${root}/cmd/cairnline"

  cp "${root}/README.md" "${root}/LICENSE" "${work}/"
  tar -C "${work}" -czf "${dist}/${name}.tar.gz" cairnline README.md LICENSE
done

(cd "${dist}" && shasum -a 256 cairnline_*.tar.gz > checksums.txt)
rm -rf "${tmp}"
