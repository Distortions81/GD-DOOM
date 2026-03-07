#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/bench.prof"
WAD_PATH="${ROOT_DIR}/doom.wad"
DEMO_PATH="${ROOT_DIR}/newdemo.demo"
MAP_NAME="E1M1"
OUT_DIR="${ROOT_DIR}/profiles"

usage() {
  cat <<'EOF'
Run a deterministic GD-DOOM demo with CPU profiling.

Usage:
  scripts/demo_profile.sh [options] [-- <extra gddoom flags>]

Options:
  --wad <path>       IWAD path (default: ./DOOM1.WAD)
  --demo <path>      Demo script path (default: ./e1m1.demo)
  --map <name>       Map name (default: E1M1)
  --out <dir>        Output directory for profiles/logs (default: ./profiles)
  --bin <path>       Override built binary path (default: ./.tmp/gddoom-profile)
  -h, --help         Show this help

Examples:
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
  go build -o "${BIN_PATH}" ./cmd/gddoom
)

STAMP="$(date +%Y%m%d-%H%M%S)"
CPU_PROFILE="${OUT_DIR}/demo-${MAP_NAME}-${STAMP}.cpu.prof"
RUN_LOG="${OUT_DIR}/demo-${MAP_NAME}-${STAMP}.log"

CMD=(
  "${BIN_PATH}"
  -render
  -wad "${WAD_PATH}"
  -map "${MAP_NAME}"
  -demo "${DEMO_PATH}"
  -width 3840
  -height 2160
  -cpuprofile "${CPU_PROFILE}"
   -no-vsync
)
if [[ ${#EXTRA_FLAGS[@]} -gt 0 ]]; then
  CMD+=("${EXTRA_FLAGS[@]}")
fi

echo "Running demo profile..."
echo "Command: ${CMD[*]}"
"${CMD[@]}" 2>&1 | tee "${RUN_LOG}"

echo
echo "CPU profile: ${CPU_PROFILE}"
echo "Run log:     ${RUN_LOG}"
echo
echo "Open interactive pprof UI:"
echo "  go tool pprof -http=:0 \"${BIN_PATH}\" \"${CPU_PROFILE}\""
echo
echo "Top functions:"
echo "  go tool pprof -top \"${BIN_PATH}\" \"${CPU_PROFILE}\""
