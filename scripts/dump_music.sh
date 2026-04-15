#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/out/music-dump"
DOOMU_PATH=""
DOOM2_PATH=""
SONG_NAME=""
MODE="impsynth"
SEARCH_DIRS=()

usage() {
  cat <<'EOF'
Dump Doom music into the repo's standard export layout.

Usage:
  scripts/dump_music.sh [options]

Options:
  --doomu <path>   Path to DOOMU.WAD or DOOM.WAD
  --doom2 <path>   Path to DOOM2.WAD
  --doom1 <path>   Deprecated alias for --doomu
  --search-dir <dir>
                   Extra directory to search for IWADs; may be repeated
  --out <dir>      Output directory (default: ./out/music-dump)
  --song <lump>    Export one exact music lump, e.g. D_E1M1 or D_RUNNIN
  --mode <name>    Export mode
                   (impsynth|pcspeaker|pcspeaker-clean|pcspeaker-piezo; default: impsynth)
  -h, --help       Show this help

Behavior:
  - auto-detects IWADs from ./wads first, then nearby common WAD folders
  - default full dumps use the app's built-in music export flow, which writes:
      <out>/DOOMU/<renderer>/...
      <out>/DOOM2/<renderer>/...
  - direct musicwav modes (--song or PC speaker variants) are placed into the same
    WAD/renderer folder layout so the MP3/MP4 helper scripts still work

Examples:
  scripts/dump_music.sh
  scripts/dump_music.sh --doomu ~/wads/DOOMU.WAD --doom2 ~/wads/DOOM2.WAD
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

append_search_dir() {
  local dir="$1"
  if [[ -n "${dir}" ]]; then
    SEARCH_DIRS+=("${dir}")
  fi
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

renderer_dir_for_mode() {
  case "$1" in
    impsynth)
      printf 'OPL\n'
      ;;
    pcspeaker)
      printf 'PCSPEAKER\n'
      ;;
    pcspeaker-clean)
      printf 'PCSPEAKER-CLEAN\n'
      ;;
    pcspeaker-piezo)
      printf 'PCSPEAKER-PIEZO\n'
      ;;
    *)
      echo "Unsupported mode: $1" >&2
      exit 2
      ;;
  esac
}

run_builtin_dump() {
  local wad_path="$1"
  echo "dump-music: ${wad_path}"
  (
    cd "${ROOT_DIR}"
    exec go run . -wad "${wad_path}" -dump-music -dump-music-dir "${OUT_DIR}"
  )
}

move_musicwav_target() {
  local stage_dir="$1"
  local source_label="$2"
  local wad_label="$3"
  local renderer_label="$4"
  local src_dir="${stage_dir}/${source_label}"
  local dst_dir="${OUT_DIR}/${wad_label}/${renderer_label}"
  local src

  if [[ ! -d "${src_dir}" ]]; then
    return 0
  fi

  mkdir -p "${dst_dir}"
  shopt -s nullglob
  for src in "${src_dir}"/*.wav; do
    mv "${src}" "${dst_dir}/$(basename "${src}")"
  done
  shopt -u nullglob
}

run_musicwav_target() {
  local wad_path="$1"
  local source_flag="$2"
  local source_label="$3"
  local wad_label="$4"
  local renderer_label="$5"
  local stage_dir
  local -a cmd

  stage_dir="$(mktemp -d "${OUT_DIR}/.musicwav-staging.${wad_label}.XXXXXX")"
  cmd=(
    go run ./cmd/musicwav
    -out "${stage_dir}"
    -mode "${MODE}"
    -doom1 ""
    -doom2 ""
  )
  cmd+=("${source_flag}" "${wad_path}")
  if [[ -n "${SONG_NAME}" ]]; then
    cmd+=(-song "${SONG_NAME}")
  fi

  local status=0
  if (
    cd "${ROOT_DIR}"
    exec "${cmd[@]}"
  ); then
    status=0
  else
    status=$?
  fi
  if [[ "${status}" -eq 0 ]]; then
    move_musicwav_target "${stage_dir}" "${source_label}" "${wad_label}" "${renderer_label}"
    rm -rf "${stage_dir}"
    return 0
  fi
  if [[ -n "${SONG_NAME}" ]]; then
    echo "Skipping ${wad_label}: lump ${SONG_NAME} not found there"
    rm -rf "${stage_dir}"
    return 0
  fi
  rm -rf "${stage_dir}"
  return "${status}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --doomu|--doom|--doom1)
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

append_search_dir "${ROOT_DIR}/wads"
append_search_dir "${ROOT_DIR}"
append_search_dir "$(dirname "${ROOT_DIR}")/wads"
append_search_dir "$(dirname "${ROOT_DIR}")"
if [[ -n "${HOME:-}" ]]; then
  append_search_dir "${HOME}/wads"
  append_search_dir "${HOME}/WADs"
  append_search_dir "${HOME}"
fi

DOOMU_PATH="$(resolve_wad "${DOOMU_PATH}" "DOOMU.WAD" "doomu.wad" "DOOM.WAD" "doom.wad")"
DOOM2_PATH="$(resolve_wad "${DOOM2_PATH}" "DOOM2.WAD" "doom2.wad")"

if [[ -z "${DOOMU_PATH}" && -z "${DOOM2_PATH}" ]]; then
  echo "No DOOMU/DOOM2 IWADs found. Checked the repo wads dir and common nearby WAD folders." >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

echo "Output dir: ${OUT_DIR}"
if [[ -n "${DOOMU_PATH}" ]]; then
  echo "DOOMU:      ${DOOMU_PATH}"
fi
if [[ -n "${DOOM2_PATH}" ]]; then
  echo "DOOM2:      ${DOOM2_PATH}"
fi

if [[ "${MODE}" == "impsynth" && -z "${SONG_NAME}" ]]; then
  if [[ -n "${DOOMU_PATH}" ]]; then
    run_builtin_dump "${DOOMU_PATH}"
  fi
  if [[ -n "${DOOM2_PATH}" ]]; then
    run_builtin_dump "${DOOM2_PATH}"
  fi
  exit 0
fi

RENDERER_LABEL="$(renderer_dir_for_mode "${MODE}")"
generated=0

if [[ -n "${DOOMU_PATH}" ]]; then
  run_musicwav_target "${DOOMU_PATH}" "-doom1" "doom1" "DOOMU" "${RENDERER_LABEL}"
  if [[ -d "${OUT_DIR}/DOOMU/${RENDERER_LABEL}" ]]; then
    generated=1
  fi
fi

if [[ -n "${DOOM2_PATH}" ]]; then
  run_musicwav_target "${DOOM2_PATH}" "-doom2" "doom2" "DOOM2" "${RENDERER_LABEL}"
  if [[ -d "${OUT_DIR}/DOOM2/${RENDERER_LABEL}" ]]; then
    generated=1
  fi
fi

if [[ "${generated}" -eq 0 ]]; then
  echo "No files were written." >&2
  exit 1
fi
