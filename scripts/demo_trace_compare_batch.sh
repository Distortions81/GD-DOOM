#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPARE_SCRIPT="${ROOT_DIR}/scripts/demo_trace_compare.sh"
GDDOOM_BIN="${ROOT_DIR}/.tmp/gddoom-demotrace"

DEMO_DIR="${ROOT_DIR}/demos/doom2-uvmax-late"
WAD_PATH="${ROOT_DIR}/wads/DOOM2.WAD"
OUT_ROOT="${ROOT_DIR}/tmp/doom2-uvmax-late"
REFERENCE_BIN="${ROOT_DIR}/../doom-source/linuxdoom-1.10/linux/linuxxdoom"
USE_XVFB=auto
STOP_AFTER_TICS=0
DEMO_EXIT_ON_DEATH=0
EXTRA_ARGS=()

usage() {
  cat <<'EOF'
Run scripts/demo_trace_compare.sh across a directory of external demos and summarize results.

Usage:
  scripts/demo_trace_compare_batch.sh [options] [-- <extra gddoom flags>]

Options:
  --demo-dir <path>   Directory containing .lmp files
                      (default: ./demos/doom2-uvmax-late)
  --wad <path>        IWAD path
                      (default: ./wads/DOOM2.WAD)
  --out-root <dir>    Root output directory for per-demo artifacts and summary
                      (default: ./tmp/doom2-uvmax-late)
  --ref-bin <path>    Override the original DOOM binary path
  --headless          Force xvfb-run for GD-DOOM
  --no-headless       Do not use xvfb-run; require an existing desktop display
  --stop-after-tics <n>
                      Stop GD-DOOM after <n> processed demo tics (default: 0, disabled)
  --demo-exit-on-death Exit GD-DOOM immediately on player death during demo playback
  -h, --help          Show this help

Examples:
  scripts/demo_trace_compare_batch.sh
  scripts/demo_trace_compare_batch.sh --demo-dir ./demos/doom2-uvmax-late --headless
  scripts/demo_trace_compare_batch.sh --stop-after-tics 3000 -- -render=false
EOF
}

sanitize_name() {
  local name="$1"
  name="${name%.lmp}"
  name="${name// /-}"
  name="${name//[^A-Za-z0-9._-]/-}"
  printf '%s\n' "${name}"
}

demo_lump_name() {
  local demo_path="$1"
  local base

  base="$(basename "${demo_path}")"
  if [[ "${base}" =~ MAP([0-9]{2}) ]]; then
    printf 'm%02duvmax\n' "${BASH_REMATCH[1]#0}"
    return
  fi

  base="${base%.lmp}"
  base="${base^^}"
  base="${base//[^A-Z0-9]/}"
  printf '%s\n' "${base:0:8}"
}

compare_status() {
  local compare_log="$1"
  if [[ ! -f "${compare_log}" ]]; then
    printf 'missing-compare-log\n'
    return
  fi
  if rg -q '^traces match ' "${compare_log}"; then
    printf 'match\n'
    return
  fi
  if rg -q '^mismatch ' "${compare_log}"; then
    printf 'mismatch\n'
    return
  fi
  printf 'error\n'
}

first_report_line() {
  local compare_log="$1"
  local status="$2"
  case "${status}" in
    match)
      rg -m 1 '^traces match ' "${compare_log}" || true
      ;;
    mismatch)
      rg -m 1 '^mismatch ' "${compare_log}" || true
      ;;
    *)
      sed -n '1p' "${compare_log}" 2>/dev/null || true
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --demo-dir)
      DEMO_DIR="$2"
      shift 2
      ;;
    --wad)
      WAD_PATH="$2"
      shift 2
      ;;
    --out-root)
      OUT_ROOT="$2"
      shift 2
      ;;
    --ref-bin)
      REFERENCE_BIN="$2"
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
    --stop-after-tics)
      STOP_AFTER_TICS="$2"
      shift 2
      ;;
    --demo-exit-on-death)
      DEMO_EXIT_ON_DEATH=1
      shift
      ;;
    --)
      shift
      EXTRA_ARGS+=("$@")
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

if [[ ! -d "${DEMO_DIR}" ]]; then
  echo "Demo directory not found: ${DEMO_DIR}" >&2
  exit 1
fi
if [[ ! -f "${WAD_PATH}" ]]; then
  echo "WAD not found: ${WAD_PATH}" >&2
  exit 1
fi
if [[ ! -x "${REFERENCE_BIN}" ]]; then
  echo "Reference runtime not found or not executable: ${REFERENCE_BIN}" >&2
  exit 1
fi

mapfile -t DEMOS < <(find "${DEMO_DIR}" -maxdepth 1 -type f -name '*.lmp' | sort)
if [[ ${#DEMOS[@]} -eq 0 ]]; then
  echo "No .lmp demos found in ${DEMO_DIR}" >&2
  exit 1
fi

mkdir -p "${ROOT_DIR}/.tmp" "${OUT_ROOT}"

echo "Preparing GD-DOOM trace binary: ${GDDOOM_BIN}"
(cd "${ROOT_DIR}" && go build -o "${GDDOOM_BIN}" .)

SUMMARY="${OUT_ROOT}/summary.tsv"
printf 'demo\tlump\tstatus\treport\tout_dir\n' >"${SUMMARY}"

for demo_path in "${DEMOS[@]}"; do
  demo_name="$(basename "${demo_path}")"
  lump_name="$(demo_lump_name "${demo_path}")"
  safe_name="$(sanitize_name "${demo_name}")"
  run_out="${OUT_ROOT}/${safe_name%.lmp}"

  echo
  echo "=== ${demo_name} (${lump_name}) ==="

  cmd=(
    "${COMPARE_SCRIPT}"
    --wad "${WAD_PATH}"
    --demo-lump "${lump_name}"
    --demo "${demo_path}"
    --out "${run_out}"
    --ref-bin "${REFERENCE_BIN}"
    --gd-bin "${GDDOOM_BIN}"
    --keep-going
  )
  if [[ "${USE_XVFB}" == "yes" ]]; then
    cmd+=(--headless)
  elif [[ "${USE_XVFB}" == "no" ]]; then
    cmd+=(--no-headless)
  fi
  if [[ "${STOP_AFTER_TICS}" != "0" ]]; then
    cmd+=(--stop-after-tics "${STOP_AFTER_TICS}")
  fi
  if [[ "${DEMO_EXIT_ON_DEATH}" == "1" ]]; then
    cmd+=(--demo-exit-on-death)
  fi
  if [[ ${#EXTRA_ARGS[@]} -gt 0 ]]; then
    cmd+=(-- "${EXTRA_ARGS[@]}")
  fi

  set +e
  "${cmd[@]}"
  rc=$?
  set -e

  compare_log="${run_out}/compare.log"
  status="$(compare_status "${compare_log}")"
  report="$(first_report_line "${compare_log}" "${status}")"
  if [[ -z "${report}" ]]; then
    report="exit=${rc}"
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "${demo_name}" "${lump_name}" "${status}" "${report}" "${run_out}" >>"${SUMMARY}"
  printf 'status=%s report=%s\n' "${status}" "${report}"
done

echo
echo "Summary written to ${SUMMARY}"
column -t -s $'\t' "${SUMMARY}"
