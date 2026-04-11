#!/usr/bin/env bash
set -euo pipefail

exec go test -tags=integration "$@"
