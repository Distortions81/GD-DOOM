# GD-DOOM browser audio patch

This is `github.com/ebitengine/oto/v3` **v3.5.0**, copied from the Go module
distribution. The Apache 2.0 license and upstream source headers are preserved.
The unused `example/` program is omitted. Native platform implementations and
the upstream mixer are unchanged.

The root `go.mod` replaces Oto with this local module so regular builds, tests,
and WASM builds all use the same implementation. Ebitengine is not forked.

The only upstream implementation change is in `driver_js.go`: its AudioWorklet
processor is extracted into the embedded `driver_worklet.js` and the request
handler supplies at most 1024 frames per reply. The ScriptProcessor fallback
is unchanged.

The shared browser output now starts with about 60 ms of queued audio and
requests another block with about 37 ms remaining at 44.1 kHz. It measures
request/response delays on the audio thread, grows headroom quickly when
responses are late or playback underruns, and reduces it gradually after ten
seconds of stable responses. The ring buffer is capped below 200 ms. Changing
its target does not discard or repeat samples. Increased headroom also increases
sound-effect latency while the browser is under load; it cannot protect against
an arbitrarily long main-thread stall.

`driver_worklet.test.cjs` runs the actual embedded processor with a deterministic
audio clock and simulated main-thread replies:

```sh
node third_party/oto/driver_worklet.test.cjs
```

When updating Oto, copy the new upstream release, restore the small worklet
integration in `driver_js.go`, retain these three added files, and run the Node
tests, native Go tests, and a WASM build. Keep the pinned version in this note and
the root `go.mod` synchronized.
