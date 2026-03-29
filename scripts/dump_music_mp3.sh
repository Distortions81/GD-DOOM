#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IN_DIR="${ROOT_DIR}/out/music-dump"
OUT_DIR=""

usage() {
  cat <<'EOF'
Convert dumped music WAV files to 256k MP3s.

Usage:
  scripts/dump_music_mp3.sh [options]

Options:
  --in <dir>       Input dump directory (default: ./out/music-dump)
  --out <dir>      Output directory (default: same as input)
  -h, --help       Show this help

Notes:
  - Requires ffmpeg in PATH.
  - Preserves the relative folder structure.
  - Writes .mp3 files alongside the .wav files unless --out is set.

Examples:
  scripts/dump_music_mp3.sh
  scripts/dump_music_mp3.sh --in ./out/music-dump --out ./out/music-mp3
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --in)
      IN_DIR="$2"
      shift 2
      ;;
    --out)
      OUT_DIR="$2"
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

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required but was not found in PATH" >&2
  exit 1
fi

if [[ ! -d "${IN_DIR}" ]]; then
  echo "Input directory does not exist: ${IN_DIR}" >&2
  exit 1
fi

if [[ -z "${OUT_DIR}" ]]; then
  OUT_DIR="${IN_DIR}"
fi

mapfile -d '' WAV_FILES < <(find "${IN_DIR}" -type f \( -iname '*.wav' \) -print0 | sort -z)

if [[ ${#WAV_FILES[@]} -eq 0 ]]; then
  echo "No WAV files found under ${IN_DIR}" >&2
  exit 1
fi

for src in "${WAV_FILES[@]}"; do
  rel="${src#"${IN_DIR}/"}"
  if [[ "${src}" == "${rel}" ]]; then
    rel="$(basename "${src}")"
  fi
  dst_rel="${rel%.*}.mp3"
  dst="${OUT_DIR}/${dst_rel}"
  mkdir -p "$(dirname "${dst}")"
  echo "mp3 ${rel} -> ${dst}"
  ffmpeg -v error -y -i "${src}" -codec:a libmp3lame -b:a 256k "${dst}"
done

echo "Converted ${#WAV_FILES[@]} file(s) to MP3 under ${OUT_DIR}"
