#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="${ROOT_DIR}/.tmp/gddoom-occlusion-bench"
WAD_PATH="${ROOT_DIR}/doom.wad"
DEMO_PATH="${ROOT_DIR}/e1m1.demo"
MAP_NAME="E1M1"
OUT_DIR="${ROOT_DIR}/profiles"
WIDTH=3840
HEIGHT=2160
DETAIL_LEVEL=2
SOURCEPORT_MODE=1
REPEATS=3

usage() {
  cat <<'USAGE'
Sweep GD-DOOM occlusion/clipping combinations with the built-in demo benchmark.

Defaults:
  resolution: 3840x2160
  detail:     0
  map:        E1M1
  demo:       ./e1m1.demo
  mode:       sourceport on
  repeats:    3
  renderers:  doom-basic, unified-bsp

Usage:
  scripts/bench_occlusion_matrix.sh [options] [-- <extra gddoom flags>]

Options:
  --wad <path>         IWAD path (default: ./doom.wad)
  --demo <path>        Demo script path (default: ./e1m1.demo)
  --map <name>         Map name (default: E1M1)
  --out <dir>          Output directory (default: ./profiles)
  --width <px>         Render width (default: 3840)
  --height <px>        Render height (default: 2160)
  --detail <n>         Detail level (default: 0)
  --repeats <n>        Runs per combination to average (default: 3)
  --no-sourceport      Do not pass -sourceport-mode
  --bin <path>         Override built binary path
  -h, --help           Show this help
USAGE
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
    --width)
      WIDTH="$2"
      shift 2
      ;;
    --height)
      HEIGHT="$2"
      shift 2
      ;;
    --detail)
      DETAIL_LEVEL="$2"
      shift 2
      ;;
    --repeats)
      REPEATS="$2"
      shift 2
      ;;
    --no-sourceport)
      SOURCEPORT_MODE=0
      shift
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

STAMP="$(date +%Y%m%d-%H%M%S)"
CSV_PATH="${OUT_DIR}/occlusion-matrix-${MAP_NAME}-${STAMP}.csv"
LOG_DIR="${OUT_DIR}/occlusion-matrix-${MAP_NAME}-${STAMP}.logs"
mkdir -p "${LOG_DIR}"

echo "Building benchmark binary: ${BIN_PATH}"
(
  cd "${ROOT_DIR}"
  go build -o "${BIN_PATH}" ./cmd/gddoom
)

echo "label,renderer,wall_occlusion,wall_span_reject,wall_span_clip,wall_slice_occlusion,depth_occlusion,runs,avg_draws,best_draws,worst_draws,avg_fps,avg_elapsed_s,best_fps,worst_fps,log_dir" > "${CSV_PATH}"

run_case() {
  local renderer="$1"
  local wall_occ="$2"
  local span_reject="$3"
  local span_clip="$4"
  local slice_occ="$5"
  local depth_occ="$6"

  local label="${renderer}_wo${wall_occ}_sr${span_reject}_sc${span_clip}_so${slice_occ}_do${depth_occ}"
  local case_dir="${LOG_DIR}/${label}"
  mkdir -p "${case_dir}"

  local sum_fps=0
  local sum_elapsed=0
  local sum_draws=0
  local best_fps=0
  local worst_fps=1000000
  local best_draws=0
  local worst_draws=1000000000
  local ok_runs=0

  for ((run=1; run<=REPEATS; run++)); do
    local log_path="${case_dir}/run-${run}.log"
    local cmd=(
      "${BIN_PATH}"
      -render
      -wad "${WAD_PATH}"
      -map "${MAP_NAME}"
      -demo "${DEMO_PATH}"
#      -width "${WIDTH}"
#      -height "${HEIGHT}"
      -detail-level "${DETAIL_LEVEL}"
      -no-vsync
      "-wall-occlusion=${wall_occ}"
      "-wall-span-reject=${span_reject}"
      "-wall-span-clip=${span_clip}"
      "-wall-slice-occlusion=${slice_occ}"
      "-depth-occlusion=${depth_occ}"
    )

    if [[ "${SOURCEPORT_MODE}" == "1" ]]; then
      cmd+=(-sourceport-mode)
    fi
    cmd+=(-walk-renderer "${renderer}")
    if [[ ${#EXTRA_FLAGS[@]} -gt 0 ]]; then
      cmd+=("${EXTRA_FLAGS[@]}")
    fi

    echo "Running ${label} (${run}/${REPEATS})"
    "${cmd[@]}" > "${log_path}" 2>&1

    local bench
    bench="$(grep 'demo-bench ' "${log_path}" | tail -n 1 || true)"
    if [[ -z "${bench}" ]]; then
      continue
    fi

    local fps elapsed draws
    fps="$(sed -n 's/.* fps=\([0-9.]*\) .*/\1/p' <<<"${bench}")"
    elapsed="$(sed -n 's/.* elapsed=\([^ ]*\) tps=.*/\1/p' <<<"${bench}")"
    draws="$(sed -n 's/.* draws=\([0-9]*\) elapsed=.*/\1/p' <<<"${bench}")"
    elapsed="${elapsed%s}"

    sum_draws="$(awk -v a="${sum_draws}" -v b="${draws}" 'BEGIN { printf "%.0f", a + b }')"
    sum_fps="$(awk -v a="${sum_fps}" -v b="${fps}" 'BEGIN { printf "%.6f", a + b }')"
    sum_elapsed="$(awk -v a="${sum_elapsed}" -v b="${elapsed}" 'BEGIN { printf "%.6f", a + b }')"
    best_draws="$(awk -v a="${best_draws}" -v b="${draws}" 'BEGIN { if (b > a) printf "%.0f", b; else printf "%.0f", a }')"
    worst_draws="$(awk -v a="${worst_draws}" -v b="${draws}" 'BEGIN { if (b < a) printf "%.0f", b; else printf "%.0f", a }')"
    best_fps="$(awk -v a="${best_fps}" -v b="${fps}" 'BEGIN { if (b > a) printf "%.6f", b; else printf "%.6f", a }')"
    worst_fps="$(awk -v a="${worst_fps}" -v b="${fps}" 'BEGIN { if (b < a) printf "%.6f", b; else printf "%.6f", a }')"
    ok_runs=$((ok_runs + 1))
  done

  if [[ "${ok_runs}" -eq 0 ]]; then
    echo "${label},${renderer},${wall_occ},${span_reject},${span_clip},${slice_occ},${depth_occ},0,ERROR,ERROR,ERROR,ERROR,ERROR,ERROR,ERROR,${case_dir}" >> "${CSV_PATH}"
    return
  fi

  local avg_draws avg_fps avg_elapsed
  avg_draws="$(awk -v s="${sum_draws}" -v n="${ok_runs}" 'BEGIN { printf "%.1f", s / n }')"
  avg_fps="$(awk -v s="${sum_fps}" -v n="${ok_runs}" 'BEGIN { printf "%.3f", s / n }')"
  avg_elapsed="$(awk -v s="${sum_elapsed}" -v n="${ok_runs}" 'BEGIN { printf "%.3f", s / n }')"
  best_fps="$(awk -v a="${best_fps}" 'BEGIN { printf "%.3f", a }')"
  worst_fps="$(awk -v a="${worst_fps}" 'BEGIN { printf "%.3f", a }')"
  echo "${label},${renderer},${wall_occ},${span_reject},${span_clip},${slice_occ},${depth_occ},${ok_runs},${avg_draws},${best_draws},${worst_draws},${avg_fps},${avg_elapsed},${best_fps},${worst_fps},${case_dir}" >> "${CSV_PATH}"
}

for renderer in doom-basic unified-bsp; do
  for wall_occ in true false; do
    for span_reject in true false; do
      for span_clip in true false; do
        for slice_occ in true false; do
          for depth_occ in true false; do
            run_case "${renderer}" "${wall_occ}" "${span_reject}" "${span_clip}" "${slice_occ}" "${depth_occ}"
          done
        done
      done
    done
  done
done

echo

echo "CSV:  ${CSV_PATH}"
echo "Logs: ${LOG_DIR}"
