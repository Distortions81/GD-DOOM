#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOOM_SRC_DIR="${ROOT_DIR}/doom-source/linuxdoom-1.10"
BIN_PATH="${DOOM_SRC_DIR}/linux/linuxxdoom"
WAD_DIR="${ROOT_DIR}"
DEMO_NAME="demo1"
SCREEN_GEOMETRY="640x480x8"
REBUILD=0
EXTRA_ARGS=()

usage() {
  cat <<'USAGE'
Launch linuxdoom under Xvfb and run a timedemo.

Usage:
  scripts/linuxdoom_timedemo.sh [options] [-- <extra linuxdoom args>]

Options:
  --wad-dir <path>     Directory to use for DOOMWADDIR (default: repo root)
  --demo <name>        Demo name without .lmp suffix (default: demo1)
  --screen <geom>      Xvfb screen geometry (default: 640x480x8)
  --rebuild            Run make before launch
  --bin <path>         Override linuxdoom binary path
  -h, --help           Show this help

Examples:
  scripts/linuxdoom_timedemo.sh
  scripts/linuxdoom_timedemo.sh --wad-dir ./disabled-wads --demo demo2
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

if [[ ! -d "${WAD_DIR}" ]]; then
  echo "WAD directory not found: ${WAD_DIR}" >&2
  exit 1
fi

if [[ "${REBUILD}" -eq 1 ]]; then
  (
    cd "${DOOM_SRC_DIR}"
    make
  )
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "linuxdoom binary not found or not executable: ${BIN_PATH}" >&2
  echo "Run with --rebuild or build it in ${DOOM_SRC_DIR}." >&2
  exit 1
fi

exec env DOOMWADDIR="${WAD_DIR}" \
  xvfb-run -a --server-args="-screen 0 ${SCREEN_GEOMETRY}" \
  "${BIN_PATH}" -timedemo "${DEMO_NAME}" "${EXTRA_ARGS[@]}"
