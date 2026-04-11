#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/out/music-dump"
DOOM1_PATH=""
DOOM2_PATH=""
SONG_NAME=""
MODE="impsynth"

usage() {
  cat <<'EOF'
Dump Doom music lumps to WAV files.

Usage:
  scripts/dump_music.sh [options]

Options:
  --doom1 <path>   Path to Doom 1 IWAD. If omitted, auto-detects ./doom.wad, ./DOOM.WAD, ./DOOM1.WAD, ./doom1.wad
  --doom2 <path>   Path to Doom 2 IWAD. If omitted, auto-detects ./doom2.wad, ./DOOM2.WAD
  --out <dir>      Output directory (default: ./out/music-dump)
  --song <lump>    Export one exact music lump, e.g. D_E1M1 or D_RUNNIN
  --mode <name>    Export mode passed through to musicwav
                   (impsynth|pcspeaker|pcspeaker-clean|pcspeaker-piezo; default: impsynth)
  -h, --help       Show this help

Examples:
  scripts/dump_music.sh
  scripts/dump_music.sh --doom1 ~/wads/DOOM.WAD --doom2 ~/wads/DOOM2.WAD
  scripts/dump_music.sh --song D_E1M1 --out ./tmp/music
  scripts/dump_music.sh --song D_E1M1 --mode pcspeaker-clean --out ./tmp/music
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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --doom1)
      DOOM1_PATH="$2"
      shift 2
      ;;
    --doom2)
      DOOM2_PATH="$2"
      shift 2
      ;;
    --out)
      OUT_DIR="$2"
      shift 2
      ;;
    --song)
      SONG_NAME="$2"
      shift 2
      ;;
    --mode)
      MODE="$2"
      shift 2
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

if [[ -z "${DOOM1_PATH}" ]]; then
  DOOM1_PATH="$(find_first_existing \
    "${ROOT_DIR}/doom.wad" \
    "${ROOT_DIR}/DOOM.WAD" \
    "${ROOT_DIR}/DOOM1.WAD" \
    "${ROOT_DIR}/doom1.wad" || true)"
fi

if [[ -z "${DOOM2_PATH}" ]]; then
  DOOM2_PATH="$(find_first_existing \
    "${ROOT_DIR}/doom2.wad" \
    "${ROOT_DIR}/DOOM2.WAD" || true)"
fi

if [[ -z "${DOOM1_PATH}" && -z "${DOOM2_PATH}" ]]; then
  echo "No Doom IWADs found. Checked for Doom 1/2 under ${ROOT_DIR}." >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

CMD=(
  go run ./cmd/musicwav
  -out "${OUT_DIR}"
  -mode "${MODE}"
)

if [[ -n "${DOOM1_PATH}" ]]; then
  CMD+=(-doom1 "${DOOM1_PATH}")
else
  CMD+=(-doom1 "")
fi

if [[ -n "${DOOM2_PATH}" ]]; then
  CMD+=(-doom2 "${DOOM2_PATH}")
else
  CMD+=(-doom2 "")
fi

if [[ -n "${SONG_NAME}" ]]; then
  CMD+=(-song "${SONG_NAME}")
fi

echo "Output dir: ${OUT_DIR}"
if [[ -n "${DOOM1_PATH}" ]]; then
  echo "Doom 1:     ${DOOM1_PATH}"
fi
if [[ -n "${DOOM2_PATH}" ]]; then
  echo "Doom 2:     ${DOOM2_PATH}"
fi

(
  cd "${ROOT_DIR}"
  exec "${CMD[@]}"
)
