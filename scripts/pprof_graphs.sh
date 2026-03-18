#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/.tmp/gddoom-profile"
CPU_PROFILE=""
MEM_PROFILE=""
OUT_DIR="${ROOT_DIR}/profiles/graphs"
LABEL=""

usage() {
  cat <<'EOF'
Generate SVG call graphs from Go CPU and heap profiles.

Usage:
  scripts/pprof_graphs.sh [options]

Options:
  --bin <path>       Profiled binary path (default: ./.tmp/gddoom-profile)
  --cpu <path>       CPU profile path (default: latest *.cpu.prof in ./profiles)
  --mem <path>       Mem profile path (default: latest *.mem.prof in ./profiles)
  --out-dir <path>   Output directory for SVGs (default: ./profiles/graphs)
  --label <name>     Output filename prefix (default: derived from profile name)
  -h, --help         Show this help

Examples:
  scripts/pprof_graphs.sh
  scripts/pprof_graphs.sh --cpu ./profiles/run.cpu.prof --mem ./profiles/run.mem.prof
EOF
}

latest_profile() {
  local pattern="$1"
  find "${ROOT_DIR}/profiles" -maxdepth 1 -type f -name "${pattern}" -printf '%T@ %p\n' \
    | sort -nr \
    | head -n 1 \
    | cut -d' ' -f2-
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin)
      BIN_PATH="$2"
      shift 2
      ;;
    --cpu)
      CPU_PROFILE="$2"
      shift 2
      ;;
    --mem)
      MEM_PROFILE="$2"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --label)
      LABEL="$2"
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

if [[ -z "${CPU_PROFILE}" ]]; then
  CPU_PROFILE="$(latest_profile '*.cpu.prof')"
fi
if [[ -z "${MEM_PROFILE}" ]]; then
  MEM_PROFILE="$(latest_profile '*.mem.prof')"
fi

if [[ ! -x "$(command -v dot)" ]]; then
  echo "Graphviz 'dot' is required for SVG output." >&2
  exit 1
fi
if [[ ! -f "${BIN_PATH}" ]]; then
  echo "Binary not found: ${BIN_PATH}" >&2
  exit 1
fi
if [[ -z "${CPU_PROFILE}" || ! -f "${CPU_PROFILE}" ]]; then
  echo "CPU profile not found: ${CPU_PROFILE:-<none>}" >&2
  exit 1
fi
if [[ -z "${MEM_PROFILE}" || ! -f "${MEM_PROFILE}" ]]; then
  echo "Mem profile not found: ${MEM_PROFILE:-<none>}" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

if [[ -z "${LABEL}" ]]; then
  LABEL="$(basename "${CPU_PROFILE}" .cpu.prof)"
fi

CPU_SVG="${OUT_DIR}/${LABEL}-cpu.svg"
MEM_SVG="${OUT_DIR}/${LABEL}-mem.svg"

echo "Generating CPU graph..."
go tool pprof -dot "${BIN_PATH}" "${CPU_PROFILE}" | dot -Tsvg > "${CPU_SVG}"

echo "Generating memory graph..."
go tool pprof -dot -inuse_space "${BIN_PATH}" "${MEM_PROFILE}" | dot -Tsvg > "${MEM_SVG}"

echo "CPU SVG: ${CPU_SVG}"
echo "Mem SVG: ${MEM_SVG}"
