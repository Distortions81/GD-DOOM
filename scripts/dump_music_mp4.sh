#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IN_DIR="${ROOT_DIR}/out/music-dump"
OUT_DIR=""
FPS="auto"
VIDEO_CODEC="libx264"
AUDIO_CODEC="aac"
AUDIO_BITRATE="320k"
PRESET="veryslow"
CRF="23"
VIDEO_TUNE="stillimage"
AUTO_FPS="1"
GOP_SECONDS="30"

usage() {
  cat <<'EOF'
Convert dumped music WAV files to YouTube-friendly 1920x1080 MP4 videos.

Each video uses the per-track PNG when present:
  <dump>/<WAD>/<RENDERER>/<track>.png

Fallback:
  <dump>/<WAD>/splash.png

WAV input examples:
  <dump>/DOOM2/OPL/MAP01-running-from-evil.wav
  <dump>/DOOM2/MIDI-SC55/MAP01-running-from-evil.wav

Output examples:
  <dump>/DOOM2/OPL/MAP01-running-from-evil.mp4
  <dump>/DOOM2/MIDI-SC55/MAP01-running-from-evil.mp4

Usage:
  scripts/dump_music_mp4.sh [options]

Options:
  --in <dir>       Input dump directory (default: ./out/music-dump)
  --out <dir>      Output directory (default: same as input)
  --fps <n|auto>   Output frame rate (default: auto => 1 fps for still images)
  -h, --help       Show this help

Notes:
  - Requires ffmpeg in PATH.
  - Preserves relative folder structure.
  - Writes .mp4 files alongside the .wav files unless --out is set.
  - Uses per-track PNGs when available.
  - Falls back to splash.png inside each WAD root folder.
  - Auto FPS is tuned for static-image videos to minimize video bitrate.

Examples:
  scripts/dump_music_mp4.sh
  scripts/dump_music_mp4.sh --in ./out/music-dump --out ./out/music-video
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
    --fps)
      FPS="$2"
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

if [[ "${FPS}" == "auto" ]]; then
  OUTPUT_FPS="${AUTO_FPS}"
elif [[ "${FPS}" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  OUTPUT_FPS="${FPS}"
else
  echo "Invalid --fps value: ${FPS} (expected a number or 'auto')" >&2
  exit 2
fi

KEYINT="$(awk -v fps="${OUTPUT_FPS}" -v seconds="${GOP_SECONDS}" 'BEGIN {
  value = int((fps * seconds) + 0.5)
  if (value < 1) value = 1
  print value
}')"

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

count=0
for src in "${WAV_FILES[@]}"; do
  rel="${src#"${IN_DIR}/"}"
  if [[ "${src}" == "${rel}" ]]; then
    rel="$(basename "${src}")"
  fi

  wad_name="${rel%%/*}"
  if [[ -z "${wad_name}" || "${wad_name}" == "${rel}" ]]; then
    echo "Skipping malformed path: ${src}" >&2
    continue
  fi

  cover="${src%.*}.png"
  splash="${IN_DIR}/${wad_name}/splash.png"
  image_path=""
  if [[ -f "${cover}" ]]; then
    image_path="${cover}"
  elif [[ -f "${splash}" ]]; then
    image_path="${splash}"
  else
    echo "Skipping ${rel}: missing cover ${cover} and splash ${splash}" >&2
    continue
  fi

  dst_rel="${rel%.*}.mp4"
  dst="${OUT_DIR}/${dst_rel}"
  mkdir -p "$(dirname "${dst}")"

  echo "mp4 ${rel} -> ${dst}"
  ffmpeg -v error -y \
    -loop 1 -framerate "${OUTPUT_FPS}" -i "${image_path}" \
    -i "${src}" \
    -vf "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:black,format=yuv420p" \
    -c:v "${VIDEO_CODEC}" -preset "${PRESET}" -tune "${VIDEO_TUNE}" -crf "${CRF}" \
    -x264-params "keyint=${KEYINT}:min-keyint=${KEYINT}:scenecut=0" \
    -c:a "${AUDIO_CODEC}" -b:a "${AUDIO_BITRATE}" \
    -r "${OUTPUT_FPS}" \
    -shortest \
    -movflags +faststart \
    "${dst}"
  count=$((count + 1))
done

if [[ ${count} -eq 0 ]]; then
  echo "No MP4 files were written" >&2
  exit 1
fi

echo "Converted ${count} file(s) to MP4 under ${OUT_DIR}"
