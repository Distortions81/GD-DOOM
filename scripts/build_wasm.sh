#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-${ROOT_DIR}/build/wasm}"
GOROOT_PATH="$(go env GOROOT)"
WASM_EXEC_JS="${GOROOT_PATH}/lib/wasm/wasm_exec.js"
WASM_OPT_MODE="${WASM_OPT:-auto}"
WASM_OPT_LEVEL="${WASM_OPT_LEVEL:--O2}"
WASM_OPT_FEATURES="${WASM_OPT_FEATURES:---all-features}"

if [[ ! -f "${ROOT_DIR}/DOOM1.WAD" ]]; then
  echo "missing ${ROOT_DIR}/DOOM1.WAD" >&2
  exit 1
fi

if [[ ! -f "${WASM_EXEC_JS}" ]]; then
  echo "wasm_exec.js not found at ${WASM_EXEC_JS}" >&2
  exit 1
fi

use_wasm_opt=true
case "${WASM_OPT_MODE}" in
  auto)
    if command -v wasm-opt >/dev/null 2>&1; then
      use_wasm_opt=true
    fi
    ;;
  0|false|off|disable|disabled)
    ;;
  1|true|on|enable|enabled)
    if ! command -v wasm-opt >/dev/null 2>&1; then
      echo "WASM_OPT=${WASM_OPT_MODE} but wasm-opt is not installed or not on PATH" >&2
      exit 1
    fi
    use_wasm_opt=true
    ;;
  *)
    echo "invalid WASM_OPT value: ${WASM_OPT_MODE} (expected auto, 0, or 1)" >&2
    exit 1
    ;;
esac

mkdir -p "${OUT_DIR}"
chmod -f u+w \
  "${OUT_DIR}/gddoom.wasm" \
  "${OUT_DIR}/gddoom.wasm.gz" \
  "${OUT_DIR}/wasm_exec.js" \
  "${OUT_DIR}/index.html" \
  "${OUT_DIR}/player.html" \
  "${OUT_DIR}/launch.js" \
  "${OUT_DIR}/server.go" 2>/dev/null || true
rm -f \
  "${OUT_DIR}/gddoom.wasm" \
  "${OUT_DIR}/gddoom.wasm.gz" \
  "${OUT_DIR}/wasm_exec.js" \
  "${OUT_DIR}/index.html" \
  "${OUT_DIR}/player.html" \
  "${OUT_DIR}/launch.js" \
  "${OUT_DIR}/server.go"

GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "${OUT_DIR}/gddoom.wasm" "${ROOT_DIR}"

if [[ "${use_wasm_opt}" == "true" ]]; then
  before_bytes="$(wc -c < "${OUT_DIR}/gddoom.wasm" | tr -d ' ')"
  tmp_wasm="${OUT_DIR}/gddoom.wasm.opt"
  read -r -a wasm_opt_feature_args <<< "${WASM_OPT_FEATURES}"
  wasm-opt "${WASM_OPT_LEVEL}" "${wasm_opt_feature_args[@]}" "${OUT_DIR}/gddoom.wasm" -o "${tmp_wasm}"
  mv "${tmp_wasm}" "${OUT_DIR}/gddoom.wasm"
  after_bytes="$(wc -c < "${OUT_DIR}/gddoom.wasm" | tr -d ' ')"
  echo "Applied wasm-opt ${WASM_OPT_LEVEL} ${WASM_OPT_FEATURES}: ${before_bytes} -> ${after_bytes} bytes"
else
  echo "Skipping wasm-opt (set WASM_OPT=1 to require it, or install wasm-opt for auto mode)"
fi

cp "${WASM_EXEC_JS}" "${OUT_DIR}/wasm_exec.js"
cp "${ROOT_DIR}/web/wasm/index.html" "${OUT_DIR}/index.html"
cp "${ROOT_DIR}/web/wasm/player.html" "${OUT_DIR}/player.html"
cp "${ROOT_DIR}/web/wasm/launch.js" "${OUT_DIR}/launch.js"
cp "${ROOT_DIR}/cmd/wasmserve/main.go" "${OUT_DIR}/server.go"

gzip -c "${OUT_DIR}/gddoom.wasm" > "${OUT_DIR}/gddoom.wasm.gz"

echo "WASM build written to ${OUT_DIR}"
echo "Run it with:"
echo "  cd ${OUT_DIR}"
echo "  go run ./server.go"
