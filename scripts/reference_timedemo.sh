#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REFERENCE_RUNTIME_DIR=""
BIN_PATH=""
WAD_DIR="${ROOT_DIR}"
DEMO_NAME="demo1"
SCREEN_GEOMETRY="640x480x8"
REBUILD=0
EXTRA_ARGS=()

usage() {
  cat <<'USAGE'
Launch the reference runtime under Xvfb and run a timedemo.

Usage:
  scripts/reference_timedemo.sh [options] [-- <extra runtime args>]

Options:
  --wad-dir <path>     Directory to use for DOOMWADDIR (default: repo root)
  --demo <name>        Demo name without .lmp suffix (default: demo1)
  --screen <geom>      Xvfb screen geometry (default: 640x480x8)
  --rebuild            Run make before launch
  --bin <path>         Override runtime binary path
  -h, --help           Show this help

Examples:
  scripts/reference_timedemo.sh
  scripts/reference_timedemo.sh --wad-dir ./disabled-wads --demo demo2
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --wad-dir)
      WAD_DIR="$2"
      shift 2
      ;;
    --demo)
      DEMO_NAME="$2"
      shift 2
      ;;
    --screen)
      SCREEN_GEOMETRY="$2"
      shift 2
      ;;
    --rebuild)
      REBUILD=1
      shift
      ;;
    --bin)
      BIN_PATH="$2"
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

if [[ -z "${BIN_PATH}" ]]; then
  BIN_PATH="$(find "${ROOT_DIR}" -type f -path '*/linux/linuxxdoom' | head -n 1 || true)"
fi
if [[ -n "${BIN_PATH}" ]]; then
  REFERENCE_RUNTIME_DIR="$(cd "$(dirname "${BIN_PATH}")/.." && pwd)"
fi

if [[ ! -d "${WAD_DIR}" ]]; then
  echo "WAD directory not found: ${WAD_DIR}" >&2
  exit 1
fi

if [[ "${REBUILD}" -eq 1 ]]; then
  if [[ -z "${REFERENCE_RUNTIME_DIR}" ]]; then
    echo "reference runtime source tree not found for rebuild" >&2
    exit 1
  fi
  (
    cd "${REFERENCE_RUNTIME_DIR}"
    make
  )
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "reference runtime binary not found or not executable: ${BIN_PATH}" >&2
  echo "Run with --rebuild or point --bin at a built runtime binary." >&2
  exit 1
fi

exec env DOOMWADDIR="${WAD_DIR}" \
  xvfb-run -a --server-args="-screen 0 ${SCREEN_GEOMETRY}" \
  "${BIN_PATH}" -timedemo "${DEMO_NAME}" "${EXTRA_ARGS[@]}"
