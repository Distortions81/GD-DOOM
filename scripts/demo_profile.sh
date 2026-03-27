#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/.tmp/gddoom-profile"
WAD_PATH="${ROOT_DIR}/DOOM1.WAD"
DEMO_PATH="${ROOT_DIR}/demos/DOOM1-DEMO1.lmp"
MAP_NAME="E1M5"
OUT_DIR="${ROOT_DIR}/profiles"
WITH_MEM=0

usage() {
  cat <<'EOF'
Run a deterministic GD-DOOM demo with CPU profiling.

Usage:
  scripts/demo_profile.sh [options] [-- <extra gddoom flags>]

Options:
  --wad <path>       IWAD path (default: ./DOOM1.WAD)
  --demo <path>      Doom .lmp demo path (default: ./demos/DOOM1-DEMO1.lmp)
  --map <name>       Map name (default: E1M5)
  --out <dir>        Output directory for profiles/logs (default: ./profiles)
  --bin <path>       Override built binary path (default: ./.tmp/gddoom-profile)
  --mem              Also capture heap profile and runtime memstats
  -h, --help         Show this help

Examples:
  scripts/extract_wad_demo.py
  scripts/demo_profile.sh
  scripts/demo_profile.sh --demo ./myrun.demo -- --sourceport-mode -crt-effect
EOF
}

EXTRA_FLAGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --wad)
      WAD_PATH="$2"
      shift 2
      ;;
    --demo)
      DEMO_PATH="$2"
      shift 2
      ;;
    --map)
      MAP_NAME="$2"
      shift 2
      ;;
    --out)
      OUT_DIR="$2"
      shift 2
      ;;
    --bin)
      BIN_PATH="$2"
      shift 2
      ;;
    --mem)
      WITH_MEM=1
      shift
      ;;
    --)
      shift
      EXTRA_FLAGS=("$@")
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

mkdir -p "${OUT_DIR}"
mkdir -p "$(dirname "${BIN_PATH}")"

if [[ ! -f "${WAD_PATH}" ]]; then
  echo "WAD not found: ${WAD_PATH}" >&2
  exit 1
fi
if [[ ! -f "${DEMO_PATH}" ]]; then
  echo "Demo not found: ${DEMO_PATH}" >&2
  exit 1
fi

echo "Building profiler binary: ${BIN_PATH}"
(
  cd "${ROOT_DIR}"
  go build -o "${BIN_PATH}" .
)

STAMP="$(date +%Y%m%d-%H%M%S)"
RUN_LABEL="${MAP_NAME}"
CPU_PROFILE="${OUT_DIR}/demo-${RUN_LABEL}-${STAMP}.cpu.prof"
MEM_PROFILE="${OUT_DIR}/demo-${RUN_LABEL}-${STAMP}.mem.prof"
RUN_LOG="${OUT_DIR}/demo-${RUN_LABEL}-${STAMP}.log"

CMD=(
  "${BIN_PATH}"
  -wad "${WAD_PATH}"
  #-skill 5
  #-invuln
  -sourceport-mode
  -demo-exit-on-death
  #-no-vsync
  -width 1920
  -height 1080
  #-map "${MAP_NAME}"
  -demo "${DEMO_PATH}"
  -cpuprofile "${CPU_PROFILE}"
)
if [[ ${WITH_MEM} -eq 1 ]]; then
  CMD+=(-memprofile "${MEM_PROFILE}" -memstats)
fi
if [[ ${#EXTRA_FLAGS[@]} -gt 0 ]]; then
  CMD+=("${EXTRA_FLAGS[@]}")
fi

echo "Running demo profile..."
echo "Command: ${CMD[*]}"
"${CMD[@]}" 2>&1 | tee "${RUN_LOG}"

echo
echo "CPU profile: ${CPU_PROFILE}"
if [[ ${WITH_MEM} -eq 1 ]]; then
  echo "Mem profile: ${MEM_PROFILE}"
fi
echo "Run log:     ${RUN_LOG}"
echo
echo "Open interactive pprof UI:"
echo "  go tool pprof -http=:0 \"${BIN_PATH}\" \"${CPU_PROFILE}\""
echo
echo "Top functions:"
echo "  go tool pprof -top \"${BIN_PATH}\" \"${CPU_PROFILE}\""
