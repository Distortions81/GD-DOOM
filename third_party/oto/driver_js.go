// Copyright 2021 The Oto Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oto

import (
	_ "embed"
	"errors"
	"runtime"
	"syscall/js"
	"unsafe"

	"github.com/ebitengine/oto/v3/internal/mux"
)

//go:embed driver_worklet.js
var workletScript string

type context struct {
	audioContext            js.Value
	scriptProcessor         js.Value
	scriptProcessorCallback js.Func
	ready                   bool

	mux *mux.Mux
}

func newContext(sampleRate int, channelCount int, format mux.Format, bufferSizeInBytes int, _ string) (*context, chan struct{}, error) {
	ready := make(chan struct{})

	class := js.Global().Get("AudioContext")
	if !class.Truthy() {
		class = js.Global().Get("webkitAudioContext")
	}
	if !class.Truthy() {
		return nil, nil, errors.New("oto: AudioContext or webkitAudioContext was not found")
	}
	options := js.Global().Get("Object").New()
	options.Set("sampleRate", sampleRate)

	d := &context{
		audioContext: class.New(options),
		mux:          mux.New(sampleRate, channelCount, format),
	}

	if bufferSizeInBytes == 0 {
		// 4096 was not great at least on Safari 15.
		bufferSizeInBytes = 8192 * channelCount
	}

	buf32 := make([]float32, bufferSizeInBytes/4)

	if w := d.audioContext.Get("audioWorklet"); w.Truthy() {
		// Keep individual mixer calls bounded while the worklet adapts its output
		// reservoir independently. Lower sample rates use proportionally smaller
		// blocks; the legacy ScriptProcessor buffer below stays unchanged.
		blockFrames := min(1024, max(128, (sampleRate/40/128)*128))
		workletBuf32 := make([]float32, blockFrames*channelCount)
		scriptURL := newScriptURL(workletScript)
		var onAddModuleSuccess js.Func
		onAddModuleSuccess = js.FuncOf(func(this js.Value, arguments []js.Value) any {
			js.Global().Get("URL").Call("revokeObjectURL", scriptURL)

			node := js.Global().Get("AudioWorkletNode").New(d.audioContext, "oto-worklet-processor", map[string]any{
				"outputChannelCount": []any{channelCount},
				"processorOptions": map[string]any{
					"channelCount": channelCount,
					"blockFrames":  blockFrames,
				},
			})
			port := node.Get("port")
			// When the worklet processor requests more data, send the request to the worklet.
			port.Set("onmessage", js.FuncOf(func(this js.Value, arguments []js.Value) any {
				frames := arguments[0].Get("data").Get("frames").Int()
				if frames <= 0 || frames > blockFrames {
					return nil
				}
				requested := workletBuf32[:frames*channelCount]
				d.mux.ReadFloat32s(requested)
				buf := float32SliceToTypedArray(requested)
				port.Call("postMessage", buf, map[string]any{
					"transfer": []any{buf.Get("buffer")},
				})
				return nil
			}))
			node.Call("connect", d.audioContext.Get("destination"))

			onAddModuleSuccess.Release()
			return nil
		})
		w.Call("addModule", scriptURL).Call("then", onAddModuleSuccess)
	} else {
		// Use ScriptProcessorNode if AudioWorklet is not available.

		chBuf32 := make([][]float32, channelCount)
		for i := range chBuf32 {
			chBuf32[i] = make([]float32, len(buf32)/channelCount)
		}

		sp := d.audioContext.Call("createScriptProcessor", bufferSizeInBytes/4/channelCount, 0, channelCount)
		f := js.FuncOf(func(this js.Value, arguments []js.Value) any {
			d.mux.ReadFloat32s(buf32)
			for i := 0; i < channelCount; i++ {
				for j := range chBuf32[i] {
					chBuf32[i][j] = buf32[j*channelCount+i]
				}
			}

			buf := arguments[0].Get("outputBuffer")
			if buf.Get("copyToChannel").Truthy() {
				for i := 0; i < channelCount; i++ {
					buf.Call("copyToChannel", float32SliceToTypedArray(chBuf32[i]), i, 0)
				}
			} else {
				// copyToChannel is not defined on Safari 11.
				for i := 0; i < channelCount; i++ {
					buf.Call("getChannelData", i).Call("set", float32SliceToTypedArray(chBuf32[i]))
				}
			}
			return nil
		})
		sp.Call("addEventListener", "audioprocess", f)
		d.scriptProcessor = sp
		d.scriptProcessorCallback = f
		sp.Call("connect", d.audioContext.Get("destination"))
	}

	// Browsers require user interaction to start the audio.
	// https://developers.google.com/web/updates/2017/09/autoplay-policy-changes#webaudio

	events := []string{"touchend", "keyup", "mouseup"}

	var onEventFired js.Func
	var onResumeSuccess js.Func
	onResumeSuccess = js.FuncOf(func(this js.Value, arguments []js.Value) any {
		d.ready = true
		close(ready)
		for _, event := range events {
			js.Global().Get("document").Call("removeEventListener", event, onEventFired)
		}
		onEventFired.Release()
		onResumeSuccess.Release()
		return nil
	})
	onEventFired = js.FuncOf(func(this js.Value, arguments []js.Value) any {
		if !d.ready {
			d.audioContext.Call("resume").Call("then", onResumeSuccess)
		}
		return nil
	})
	for _, event := range events {
		js.Global().Get("document").Call("addEventListener", event, onEventFired)
	}

	return d, ready, nil
}

func (c *context) Suspend() error {
	c.audioContext.Call("suspend")
	return nil
}

func (c *context) Resume() error {
	c.audioContext.Call("resume")
	return nil
}

func (c *context) Err() error {
	return nil
}

func float32SliceToTypedArray(s []float32) js.Value {
	bs := unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*4)
	a := js.Global().Get("Uint8Array").New(len(bs))
	js.CopyBytesToJS(a, bs)
	runtime.KeepAlive(s)
	buf := a.Get("buffer")
	return js.Global().Get("Float32Array").New(buf, a.Get("byteOffset"), a.Get("byteLength").Int()/4)
}

func newScriptURL(script string) js.Value {
	blob := js.Global().Get("Blob").New([]any{script}, map[string]any{"type": "text/javascript"})
	return js.Global().Get("URL").Call("createObjectURL", blob)
}
