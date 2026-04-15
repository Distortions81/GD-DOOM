#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG="./internal/doomruntime"
TEST_NAME="TestGenerateAlphaPatchBoundingBoxes"
OUT_DIR="${ROOT_DIR}/internal/doomruntime/testdata/alpha_patch_bbox_dump"
EXTRA_ARGS=()
SEARCH_DIRS=()
DOOM1_PATH=""
DOOMU_PATH=""
DOOM2_PATH=""

usage() {
  cat <<'EOF'
Generate bbox PNG dumps and manifest JSON for decodable alpha-bearing WAD patch images.

Usage:
  scripts/dump_billboard_bboxes.sh [options] [-- <extra go test args>]

Options:
  --doom1 <path>        Path to DOOM1.WAD
  --doomu <path>        Path to DOOMU.WAD or DOOM.WAD
  --doom2 <path>        Path to DOOM2.WAD
  --search-dir <dir>    Extra directory to search for IWADs; may be repeated
  --out <dir>           Output root directory
  -h, --help            Show this help

Behavior:
  - runs the billboard bbox dump test with -count=1 to avoid cached skips
  - auto-detects WADs across multiple folders
  - writes one subdirectory per IWAD under:
      internal/doomruntime/testdata/alpha_patch_bbox_dump

Examples:
  scripts/dump_billboard_bboxes.sh
  scripts/dump_billboard_bboxes.sh --search-dir ~/wads --search-dir /mnt/wads
  scripts/dump_billboard_bboxes.sh --doomu ~/wads/DOOMU.WAD --doom2 ~/wads/DOOM2.WAD
  scripts/dump_billboard_bboxes.sh -- -v
EOF
}

find_first_existing() {
  local candidate
  for candidate in "$@"; do
    if [[ -n "${candidate}" && -f "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}

append_search_dir() {
  local dir="$1"
  if [[ -z "${dir}" ]]; then
    return
  fi
  SEARCH_DIRS+=("${dir}")
}

resolve_wad() {
  local explicit="$1"
  shift
  local names=("$@")
  local dir
  local candidates=()
  if [[ -n "${explicit}" ]]; then
    printf '%s\n' "${explicit}"
    return 0
  fi
  for dir in "${SEARCH_DIRS[@]}"; do
    for name in "${names[@]}"; do
      candidates+=("${dir}/${name}")
    done
  done
  find_first_existing "${candidates[@]}" || true
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --doom1)
      DOOM1_PATH="$2"
      shift 2
      ;;
    --doomu|--doom)
      DOOMU_PATH="$2"
      shift 2
      ;;
    --doom2)
      DOOM2_PATH="$2"
      shift 2
      ;;
    --search-dir)
      append_search_dir "$2"
      shift 2
      ;;
    --out)
      OUT_DIR="$2"
      shift 2
      ;;
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

append_search_dir "${ROOT_DIR}"
append_search_dir "${ROOT_DIR}/wads"
append_search_dir "$(dirname "${ROOT_DIR}")"
append_search_dir "$(dirname "${ROOT_DIR}")/wads"
if [[ -n "${HOME:-}" ]]; then
  append_search_dir "${HOME}"
  append_search_dir "${HOME}/wads"
  append_search_dir "${HOME}/WADs"
fi

DOOM1_PATH="$(resolve_wad "${DOOM1_PATH}" "DOOM1.WAD" "doom1.wad")"
DOOMU_PATH="$(resolve_wad "${DOOMU_PATH}" "DOOMU.WAD" "doomu.wad" "DOOM.WAD" "doom.wad")"
DOOM2_PATH="$(resolve_wad "${DOOM2_PATH}" "DOOM2.WAD" "doom2.wad")"

mkdir -p "${OUT_DIR}"

run_dump() {
  local label="$1"
  local wad_path="$2"
  local target_dir="${OUT_DIR}/${label}"

  if [[ -z "${wad_path}" ]]; then
    echo "Skipping ${label}: WAD not found"
    return 0
  fi

  echo "${label}: ${wad_path}"
  rm -rf "${target_dir}"
  mkdir -p "${target_dir}"
  (
    cd "${ROOT_DIR}"
    export GD_DUMP_BILLBOARD_WAD="${wad_path}"
    export GD_DUMP_BILLBOARD_OUT_DIR="${target_dir}"
    exec go test -tags=integration "${PKG}" -run "${TEST_NAME}" -count=1 -v "${EXTRA_ARGS[@]}"
  )
}

echo "Output root: ${OUT_DIR}"

run_dump "DOOM1" "${DOOM1_PATH}"
run_dump "DOOMU" "${DOOMU_PATH}"
run_dump "DOOM2" "${DOOM2_PATH}"
