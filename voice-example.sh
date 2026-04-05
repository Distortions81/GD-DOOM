#!/usr/bin/env bash
set -euo pipefail

WAV="./sample.wav"
PIPE="/tmp/fake-mic-$$.pipe"
SOURCE_NAME="wav_mic"

[[ -f "$WAV" ]] || { echo "missing ./sample.wav" >&2; exit 1; }
command -v pactl >/dev/null || { echo "need pactl" >&2; exit 1; }
command -v ffmpeg >/dev/null || { echo "need ffmpeg" >&2; exit 1; }

SRC_MOD=""
PLAYER_PID=""

replace_existing_source() {
    local mods
    mods="$(
        pactl list short modules |
            awk -v source_name="$SOURCE_NAME" '
                $2 == "module-pipe-source" && index($0, "source_name=" source_name) > 0 { print $1 }
            '
    )"

    if [[ -z "$mods" ]]; then
        return 1
    fi

    echo "replacing existing source: $SOURCE_NAME"
    while IFS= read -r mod; do
        [[ -n "$mod" ]] || continue
        pactl unload-module "$mod" >/dev/null 2>&1 || true
    done <<< "$mods"
    return 0
}

cleanup() {
    local ec=$?

    if [[ -n "$PLAYER_PID" ]]; then
        kill "$PLAYER_PID" >/dev/null 2>&1 || true
        wait "$PLAYER_PID" 2>/dev/null || true
    fi

    if [[ -n "$SRC_MOD" ]]; then
        pactl unload-module "$SRC_MOD" >/dev/null 2>&1 || true
    fi

    rm -f "$PIPE"
    exit "$ec"
}
trap cleanup EXIT INT TERM

if pactl list short sources | awk '{print $2}' | grep -Fxq "$SOURCE_NAME"; then
    if ! replace_existing_source; then
        echo "source exists and is not a replaceable pipe source: $SOURCE_NAME" >&2
        exit 1
    fi
fi

mkfifo "$PIPE"

SRC_MOD="$(
    pactl load-module module-pipe-source \
        source_name="$SOURCE_NAME" \
        file="$PIPE" \
        format=s16le \
        rate=48000 \
        channels=1 \
        source_properties="device.description=FakeMic"
)"

echo "fake mic created: $SOURCE_NAME"
echo "select '$SOURCE_NAME' as the input device in your app"
echo "press Ctrl+C to stop"

ffmpeg -hide_banner -loglevel error -re -i "$WAV" \
    -f s16le -ac 1 -ar 48000 - > "$PIPE" &
PLAYER_PID="$!"

wait "$PLAYER_PID"
