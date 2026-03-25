#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REFERENCE_BIN="${ROOT_DIR}/../doom-source/linuxdoom-1.10/linux/linuxxdoom"
GDDOOM_BIN="${ROOT_DIR}/.tmp/gddoom-demotrace"
TRACECMP_BIN="${ROOT_DIR}/.tmp/demotracecmp"
WAD_DIR="${ROOT_DIR}"
WAD_PATH="${ROOT_DIR}/DOOM1.WAD"
DEMO_LUMP="demo1"
DEMO_PATH="${ROOT_DIR}/demos/DOOM1-DEMO1.lmp"
OUT_DIR="${ROOT_DIR}/tmp/demo-trace-compare"
KEEP_GOING=0
GDDOOM_FLAGS=()
USE_XVFB=auto
STOP_AFTER_TICS=0

usage() {
  cat <<'EOF'
Run the original DOOM source and GD-DOOM against the same demo, then compare traces.

Usage:
  scripts/demo_trace_compare.sh [options] [-- <extra gddoom flags>]

Options:
  --wad-dir <path>    Directory exposed to the reference runtime as DOOMWADDIR
                      (default: repo root)
  --wad <path>        IWAD path passed to GD-DOOM (default: ./DOOM1.WAD)
  --demo-lump <name>  Built-in demo lump name for the reference runtime
                      (default: demo1)
  --demo <path>       .lmp demo file for GD-DOOM
                      (default: ./demos/DOOM1-DEMO1.lmp)
  --out <dir>         Output directory for logs and traces
                      (default: ./tmp/demo-trace-compare)
  --ref-bin <path>    Override the original DOOM binary path
  --gd-bin <path>     Reuse an existing GD-DOOM binary instead of rebuilding
  --headless          Force xvfb-run for GD-DOOM
  --no-headless       Do not use xvfb-run; require an existing desktop display
  --keep-going        Keep artifacts even when the compare fails
  --stop-after-tics <n>
                      Stop GD-DOOM after <n> processed demo tics (default: 0, disabled)
  -h, --help          Show this help

Examples:
  scripts/demo_trace_compare.sh
  scripts/demo_trace_compare.sh --demo-lump demo2 --demo ./demos/DOOM1-DEMO2.lmp
  scripts/demo_trace_compare.sh --out ./tmp/compare-demo1 -- -no-vsync -width 640 -height 400
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --wad-dir)
      WAD_DIR="$2"
      shift 2
      ;;
    --wad)
      WAD_PATH="$2"
      shift 2
      ;;
    --demo-lump)
      DEMO_LUMP="$2"
      shift 2
      ;;
    --demo)
      DEMO_PATH="$2"
      shift 2
      ;;
    --out)
      OUT_DIR="$2"
      shift 2
      ;;
    --ref-bin)
      REFERENCE_BIN="$2"
      shift 2
      ;;
    --gd-bin)
      GDDOOM_BIN="$2"
      shift 2
      ;;
    --headless)
      USE_XVFB=yes
      shift
      ;;
    --no-headless)
      USE_XVFB=no
      shift
      ;;
    --keep-going)
      KEEP_GOING=1
      shift
      ;;
    --stop-after-tics)
      STOP_AFTER_TICS="$2"
      shift 2
      ;;
    --)
      shift
      GDDOOM_FLAGS+=("$@")
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
if [[ ! -f "${WAD_PATH}" ]]; then
  echo "WAD not found: ${WAD_PATH}" >&2
  exit 1
fi
if [[ ! -f "${DEMO_PATH}" ]]; then
  echo "Demo file not found: ${DEMO_PATH}" >&2
  exit 1
fi
if [[ ! -x "${REFERENCE_BIN}" ]]; then
  echo "Reference runtime not found or not executable: ${REFERENCE_BIN}" >&2
  exit 1
fi
mkdir -p "${OUT_DIR}"
mkdir -p "${ROOT_DIR}/.tmp"

echo "Building GD-DOOM trace binary: ${GDDOOM_BIN}"
rm -f "${GDDOOM_BIN}"
(
  cd "${ROOT_DIR}"
  go clean
  go build -o "${GDDOOM_BIN}" .
)

echo "Building trace comparator: ${TRACECMP_BIN}"
rm -f "${TRACECMP_BIN}"
(
  cd "${ROOT_DIR}"
  go clean ./cmd/demotracecmp
  go build -o "${TRACECMP_BIN}" ./cmd/demotracecmp
)

REF_TRACE="${OUT_DIR}/reference-${DEMO_LUMP}.jsonl"
REF_LOG="${OUT_DIR}/reference-${DEMO_LUMP}.log"
GD_TRACE="${OUT_DIR}/gddoom-$(basename "${DEMO_PATH}").jsonl"
GD_LOG="${OUT_DIR}/gddoom-$(basename "${DEMO_PATH}").log"
CMP_LOG="${OUT_DIR}/compare.log"

rm -f "${REF_TRACE}" "${REF_LOG}" "${GD_TRACE}" "${GD_LOG}" "${CMP_LOG}"

echo "Tracing reference runtime: lump=${DEMO_LUMP}"
env DOOMWADDIR="${WAD_DIR}" \
  "${REFERENCE_BIN}" \
  -tracedemo "${DEMO_LUMP}" \
  -tracefile "${REF_TRACE}" \
  >"${REF_LOG}" 2>&1

echo "Tracing GD-DOOM: demo=${DEMO_PATH}"
if [[ "${STOP_AFTER_TICS}" != "0" ]]; then
  GDDOOM_FLAGS+=(-demo-stop-after-tics "${STOP_AFTER_TICS}")
fi
if [[ "${USE_XVFB}" == "yes" ]]; then
  if ! command -v xvfb-run >/dev/null 2>&1; then
    echo "xvfb-run is required for --headless." >&2
    exit 1
  fi
  xvfb-run -a \
    "${GDDOOM_BIN}" \
    -wad "${WAD_PATH}" \
    -demo "${DEMO_PATH}" \
    -trace-demo-state "${GD_TRACE}" \
    -no-vsync \
    "${GDDOOM_FLAGS[@]}" \
    >"${GD_LOG}" 2>&1
elif [[ "${USE_XVFB}" == "no" ]]; then
  if [[ -z "${DISPLAY:-}" ]]; then
    echo "--no-headless requires DISPLAY to be set." >&2
    exit 1
  fi
  "${GDDOOM_BIN}" \
    -wad "${WAD_PATH}" \
    -demo "${DEMO_PATH}" \
    -trace-demo-state "${GD_TRACE}" \
    -no-vsync \
    "${GDDOOM_FLAGS[@]}" \
    >"${GD_LOG}" 2>&1
elif [[ -n "${DISPLAY:-}" ]]; then
  "${GDDOOM_BIN}" \
    -wad "${WAD_PATH}" \
    -demo "${DEMO_PATH}" \
    -trace-demo-state "${GD_TRACE}" \
    -no-vsync \
    "${GDDOOM_FLAGS[@]}" \
    >"${GD_LOG}" 2>&1
else
  if ! command -v xvfb-run >/dev/null 2>&1; then
    echo "No DISPLAY found. Install xvfb-run or rerun from a desktop session." >&2
    exit 1
  fi
  xvfb-run -a \
    "${GDDOOM_BIN}" \
    -wad "${WAD_PATH}" \
    -demo "${DEMO_PATH}" \
    -trace-demo-state "${GD_TRACE}" \
    -no-vsync \
    "${GDDOOM_FLAGS[@]}" \
    >"${GD_LOG}" 2>&1
fi

echo "Comparing traces"
set +e
"${TRACECMP_BIN}" -left "${REF_TRACE}" -right "${GD_TRACE}" | tee "${CMP_LOG}"
STATUS=${PIPESTATUS[0]}
set -e

echo "Artifacts:"
echo "  reference trace: ${REF_TRACE}"
echo "  reference log:   ${REF_LOG}"
echo "  gddoom trace:    ${GD_TRACE}"
echo "  gddoom log:      ${GD_LOG}"
echo "  compare log:     ${CMP_LOG}"

if [[ ${STATUS} -ne 0 && ${KEEP_GOING} -eq 0 ]]; then
  exit "${STATUS}"
fi

exit "${STATUS}"
