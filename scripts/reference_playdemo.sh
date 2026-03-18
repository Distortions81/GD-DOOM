#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REFERENCE_RUNTIME_DIR=""
BIN_PATH=""
WAD_DIR="${ROOT_DIR}"
DEMO_NAME="demo1"
REBUILD=0
USE_XEPHYR=1
XEPHYR_DISPLAY=":99"
XEPHYR_SCREEN="640x480x8"
EXTRA_ARGS=()

usage() {
  cat <<'USAGE'
Launch the reference runtime in a visible X11 window and play a built-in demo.

Usage:
  scripts/reference_playdemo.sh [options] [-- <extra runtime args>]

Options:
  --wad-dir <path>     Directory to use for DOOMWADDIR (default: repo root)
  --demo <name>        Demo name without .lmp suffix (default: demo1)
  --rebuild            Run make before launch
  --bin <path>         Override runtime binary path
  --direct             Run directly on DISPLAY instead of Xephyr
  --xephyr-display <d> Nested Xephyr display (default: :99)
  --xephyr-screen <g>  Xephyr screen geometry (default: 640x480x8)
  -h, --help           Show this help

Examples:
  scripts/reference_playdemo.sh
  scripts/reference_playdemo.sh --wad-dir ./disabled-wads --demo demo2
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
    --rebuild)
      REBUILD=1
      shift
      ;;
    --direct)
      USE_XEPHYR=0
      shift
      ;;
    --bin)
      BIN_PATH="$2"
      shift 2
      ;;
    --xephyr-display)
      XEPHYR_DISPLAY="$2"
      shift 2
      ;;
    --xephyr-screen)
      XEPHYR_SCREEN="$2"
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

if [[ -z "${DISPLAY:-}" ]]; then
  echo "DISPLAY is not set. Run this from a graphical X11 session." >&2
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

if [[ "${USE_XEPHYR}" -eq 0 ]]; then
  exec env DOOMWADDIR="${WAD_DIR}" \
    "${BIN_PATH}" -playdemo "${DEMO_NAME}" "${EXTRA_ARGS[@]}"
fi

if ! command -v Xephyr >/dev/null 2>&1; then
  echo "Xephyr is required for a visible run because the reference runtime needs an 8-bit PseudoColor X server." >&2
  echo "Install it with: sudo apt install xserver-xephyr" >&2
  echo "Then rerun this script." >&2
  exit 1
fi

Xephyr "${XEPHYR_DISPLAY}" -screen "${XEPHYR_SCREEN}" -ac -reset >/dev/null 2>&1 &
XEPHYR_PID=$!
cleanup() {
  kill "${XEPHYR_PID}" >/dev/null 2>&1 || true
  wait "${XEPHYR_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
sleep 1

exec env DOOMWADDIR="${WAD_DIR}" DISPLAY="${XEPHYR_DISPLAY}" \
  "${BIN_PATH}" -playdemo "${DEMO_NAME}" "${EXTRA_ARGS[@]}"
