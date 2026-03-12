#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-${ROOT_DIR}/build/wasm}"
GOROOT_PATH="$(go env GOROOT)"
WASM_EXEC_JS="${GOROOT_PATH}/lib/wasm/wasm_exec.js"

if [[ ! -f "${ROOT_DIR}/DOOM1.WAD" ]]; then
  echo "missing ${ROOT_DIR}/DOOM1.WAD" >&2
  exit 1
fi

if [[ ! -f "${WASM_EXEC_JS}" ]]; then
  echo "wasm_exec.js not found at ${WASM_EXEC_JS}" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"
chmod -f u+w \
  "${OUT_DIR}/gddoom.wasm" \
  "${OUT_DIR}/wasm_exec.js" \
  "${OUT_DIR}/index.html" \
  "${OUT_DIR}/launch.js" \
  "${OUT_DIR}/server.go" 2>/dev/null || true
rm -f \
  "${OUT_DIR}/gddoom.wasm" \
  "${OUT_DIR}/wasm_exec.js" \
  "${OUT_DIR}/index.html" \
  "${OUT_DIR}/launch.js" \
  "${OUT_DIR}/server.go"

GOOS=js GOARCH=wasm go build -o "${OUT_DIR}/gddoom.wasm" "${ROOT_DIR}"
cp "${WASM_EXEC_JS}" "${OUT_DIR}/wasm_exec.js"
cp "${ROOT_DIR}/web/wasm/index.html" "${OUT_DIR}/index.html"
cp "${ROOT_DIR}/web/wasm/launch.js" "${OUT_DIR}/launch.js"
cp "${ROOT_DIR}/cmd/wasmserve/main.go" "${OUT_DIR}/server.go"

echo "WASM build written to ${OUT_DIR}"
echo "Run it with:"
echo "  cd ${OUT_DIR}"
echo "  go run ./server.go"
