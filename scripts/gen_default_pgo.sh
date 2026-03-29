#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/.tmp/gddoom-pgo-profile"
WAD_PATH="${ROOT_DIR}/DOOM1.WAD"
DEMO_PATH="${ROOT_DIR}/demos/DOOM1-DEMO1.lmp"
OUT_PATH="${ROOT_DIR}/default.pgo"
STOP_AFTER_TICS=1050

usage() {
  cat <<'EOF'
Profile bundled DEMO1 for 30 seconds and write ./default.pgo.

Usage:
  scripts/gen_default_pgo.sh [options] [-- <extra gddoom flags>]

Options:
  --wad <path>       IWAD path (default: ./DOOM1.WAD)
  --demo <path>      Doom .lmp demo path (default: ./demos/DOOM1-DEMO1.lmp)
  --out <path>       Output profile path (default: ./default.pgo)
  --bin <path>       Override built binary path (default: ./.tmp/gddoom-pgo-profile)
  -h, --help         Show this help

Examples:
  scripts/gen_default_pgo.sh
  scripts/gen_default_pgo.sh --out ./profiles/default-demo1.pgo
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
    --out)
      OUT_PATH="$2"
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

mkdir -p "$(dirname "${BIN_PATH}")"
mkdir -p "$(dirname "${OUT_PATH}")"

if [[ ! -f "${WAD_PATH}" ]]; then
  echo "WAD not found: ${WAD_PATH}" >&2
  exit 1
fi
if [[ ! -f "${DEMO_PATH}" ]]; then
  echo "Demo not found: ${DEMO_PATH}" >&2
  exit 1
fi

echo "Building profile binary with PGO disabled: ${BIN_PATH}"
(
  cd "${ROOT_DIR}"
  go build -pgo=off -o "${BIN_PATH}" .
)

CMD=(
  "${BIN_PATH}"
  -wad "${WAD_PATH}"
  -sourceport-mode
  -detail-level 1
  -width 1920
  -height 1080
  -demo "${DEMO_PATH}"
  -demo-stop-after-tics "${STOP_AFTER_TICS}"
  -cpuprofile "${OUT_PATH}"
)
if [[ ${#EXTRA_FLAGS[@]} -gt 0 ]]; then
  CMD+=("${EXTRA_FLAGS[@]}")
fi

echo "Running DEMO1 profile for 30 seconds (${STOP_AFTER_TICS} tics)..."
echo "Command: ${CMD[*]}"
"${CMD[@]}"

echo
echo "Wrote PGO profile: ${OUT_PATH}"
echo "Build with it via:"
echo "  go build"
