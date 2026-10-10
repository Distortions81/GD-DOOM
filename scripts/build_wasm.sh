#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-${ROOT_DIR}/build/wasm}"
GOROOT_PATH="$(go env GOROOT)"
WASM_EXEC_JS="${GOROOT_PATH}/lib/wasm/wasm_exec.js"
WASM_OPT_MODE="${WASM_OPT:-auto}"
WASM_OPT_LEVEL="${WASM_OPT_LEVEL:--O4}"
WASM_OPT_FEATURES="${WASM_OPT_FEATURES:---all-features}"
BUILD_ID="${BUILD_ID:-$(git -C "${ROOT_DIR}" rev-parse --short=12 HEAD 2>/dev/null || date +%s)}"

js_string_literal() {
  local s="${1//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
  s="${s//$'\t'/\\t}"
  printf '"%s"' "${s}"
}

# Go splits -ldflags itself, without shell-style escaping inside quotes.
# Quote the complete assignment so a configured address cannot add flags.
ldflag_string() {
  local s="$1"
  if [[ "${s}" != *[$' \t\r\n']* ]]; then
    printf '%s' "${s}"
  elif [[ "${s}" != *"'"* ]]; then
    printf "'%s'" "${s}"
  elif [[ "${s}" != *'"'* ]]; then
    printf '"%s"' "${s}"
  else
    echo 'multiplayer build address cannot contain whitespace with both quote types; URL-encode those characters' >&2
    return 1
  fi
}

WASM_SERVER_LDFLAG="$(ldflag_string "gddoom/internal/app.wasmMultiplayerServer=${MULTIPLAYER_SERVER:-}")"
WASM_LOBBY_LDFLAG="$(ldflag_string "gddoom/internal/app.wasmMultiplayerLobby=${MULTIPLAYER_LOBBY:-}")"
WASM_LDFLAGS="-s -w -X ${WASM_SERVER_LDFLAG} -X ${WASM_LOBBY_LDFLAG}"

if [[ ! -f "${ROOT_DIR}/DOOM1.WAD" ]]; then
  echo "missing ${ROOT_DIR}/DOOM1.WAD" >&2
  exit 1
fi

if [[ ! -f "${WASM_EXEC_JS}" ]]; then
  echo "wasm_exec.js not found at ${WASM_EXEC_JS}" >&2
  exit 1
fi

use_wasm_opt=false
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

echo "Cleaning Go build cache for WASM build..."
GOOS=js GOARCH=wasm go clean -cache

echo "Removing previous WASM output at ${OUT_DIR}..."
rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

GOOS=js GOARCH=wasm go build -trimpath -ldflags="${WASM_LDFLAGS}" -o "${OUT_DIR}/gddoom.wasm" "${ROOT_DIR}"

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
printf 'window.__gddoomBuildID = %s;\n' "$(js_string_literal "${BUILD_ID}")" > "${OUT_DIR}/build-id.js"
cp "${ROOT_DIR}/web/wasm/index.html" "${OUT_DIR}/index.html"
cp "${ROOT_DIR}/web/wasm/player.html" "${OUT_DIR}/player.html"
cp "${ROOT_DIR}/web/wasm/launch.js" "${OUT_DIR}/launch.js"
cat "${ROOT_DIR}/internal/render/menufont/assets/NOTICE.md" \
  "${ROOT_DIR}/internal/render/menufont/assets/UPSTREAM-LICENSE.md" > "${OUT_DIR}/font-notice.txt"
cat "${ROOT_DIR}/internal/freegames/licenses/freedoom-0.13.0/COPYING.txt" \
  "${ROOT_DIR}/internal/freegames/licenses/freedoom-0.13.0/CREDITS.txt" \
  "${ROOT_DIR}/internal/freegames/licenses/freedoom-0.13.0/CREDITS-MUSIC.txt" > "${OUT_DIR}/free-game-notices.txt"
cp "${ROOT_DIR}/cmd/wasmserve/main.go" "${OUT_DIR}/server.go"

gzip -n -f -c "${OUT_DIR}/gddoom.wasm" > "${OUT_DIR}/gddoom.wasm.gz"
# if command -v brotli >/dev/null 2>&1; then
#   brotli -f -q 11 -o "${OUT_DIR}/gddoom.wasm.br" "${OUT_DIR}/gddoom.wasm"
# else
#   echo "Skipping brotli compression (install brotli to produce gddoom.wasm.br)"
# fi

echo "WASM build written to ${OUT_DIR}"
echo "Run it with:"
echo "  cd ${OUT_DIR}"
echo "  go run ./server.go"
