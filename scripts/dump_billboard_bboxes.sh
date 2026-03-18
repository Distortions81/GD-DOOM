#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG="./internal/doomruntime"
TEST_NAME="TestGenerateAlphaPatchBoundingBoxes"
OUT_DIR="${ROOT_DIR}/internal/doomruntime/testdata/alpha_patch_bbox_dump"
EXTRA_ARGS=()

usage() {
  cat <<'EOF'
Generate bbox PNG dumps and manifest JSON for all decodable alpha-bearing WAD patch images.

Usage:
  scripts/dump_billboard_bboxes.sh [-- <extra go test args>]

Behavior:
  - runs the billboard bbox dump test with -count=1 to avoid cached skips
  - writes PNGs and manifest.json under:
      internal/doomruntime/testdata/alpha_patch_bbox_dump

Examples:
  scripts/dump_billboard_bboxes.sh
  scripts/dump_billboard_bboxes.sh -- -v
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --)
      shift
      EXTRA_ARGS=("$@")
      break
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

echo "Output dir: ${OUT_DIR}"

(
  cd "${ROOT_DIR}"
  exec go test "${PKG}" -run "${TEST_NAME}" -count=1 -v "${EXTRA_ARGS[@]}"
)
