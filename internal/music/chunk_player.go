package music

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"gddoom/internal/platformcfg"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

type playerCmdType uint8

const (
	cmdStart playerCmdType = iota
	cmdStop
	cmdClear
	cmdReset
	cmdEnqueue
	cmdSetVolume
	cmdSync
	cmdPlayStream
	cmdStopStream
	cmdClose
)

type StreamFactory func() (*StreamRenderer, error)

type playerCmd struct {
	typ            playerCmdType
	data           []byte
	vol            float64
	done           chan struct{}
	streamFactory  StreamFactory
	loop           bool
	chunkFrames    int
	enqueueBytes   int
	lookaheadBytes int
	targetBytes    int
}

// ChunkPlayer plays queued output-rate s16 stereo chunks on a dedicated goroutine.
type ChunkPlayer struct {
	ctx    *audio.Context
	player *audio.Player
	src    *pcmChunkBuffer
	volume float64

	cmds chan playerCmd
	done chan struct{}
	once sync.Once
	mu   sync.Mutex

	inline bool
	closed bool
	stream *playerStream
}

type playerStream struct {
	factory        StreamFactory
	renderer       *StreamRenderer
	loop           bool
	chunkFrames    int
	enqueueBytes   int
	lookaheadBytes int
	targetBytes    int
	started        bool
	ended          bool
}

func NewChunkPlayer() (*ChunkPlayer, error) {
	ctx := audio.CurrentContext()
	if ctx == nil {
		ctx = audio.NewContext(OutputSampleRate)
	}
	src := newPCMChunkBuffer()
	p, err := ctx.NewPlayer(src)
	if err != nil {
		return nil, err
	}
	cp := &ChunkPlayer{
		ctx:    ctx,
		player: p,
		src:    src,
		volume: 1,
		cmds:   make(chan playerCmd, chunkPlayerCommandQueueCap()),
		done:   make(chan struct{}),
		inline: platformcfg.IsWASMBuild(),
	}
	if !cp.inline {
		go cp.run()
	}
	return cp, nil
}

func (cp *ChunkPlayer) SampleRate() int {
	if cp == nil || cp.ctx == nil {
		return OutputSampleRate
	}
	return cp.ctx.SampleRate()
}

func (cp *ChunkPlayer) Start() error {
	return cp.send(playerCmd{typ: cmdStart})
}

func (cp *ChunkPlayer) Stop() error {
	return cp.send(playerCmd{typ: cmdStop})
}

func (cp *ChunkPlayer) ClearBuffer() error {
	return cp.send(playerCmd{typ: cmdClear})
}

func (cp *ChunkPlayer) ResetPlayback() error {
	return cp.sendAndWait(playerCmd{typ: cmdReset, done: make(chan struct{})})
}

func (cp *ChunkPlayer) SetVolume(v float64) error {
	if v != v {
		v = 0
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return cp.send(playerCmd{typ: cmdSetVolume, vol: v})
}

func (cp *ChunkPlayer) Sync() error {
	return cp.sendAndWait(playerCmd{typ: cmdSync, done: make(chan struct{})})
}

func (cp *ChunkPlayer) PlayStream(factory StreamFactory, loop bool, chunkFrames int, enqueueFrames int, lookaheadFrames int) error {
	if chunkFrames <= 0 {
		chunkFrames = DefaultStreamChunkFrames()
	}
	if enqueueFrames <= 0 {
		enqueueFrames = DefaultStreamEnqueueFrames()
	}
	if enqueueFrames < chunkFrames {
		enqueueFrames = chunkFrames
	}
	if lookaheadFrames <= 0 {
		lookaheadFrames = DefaultStreamLookahead()
	}
	const bytesPerFrame = 4
	lookaheadBytes := lookaheadFrames * bytesPerFrame
	targetBytes := startupPrefillBytes(lookaheadBytes)
	return cp.send(playerCmd{
		typ:            cmdPlayStream,
		streamFactory:  factory,
		loop:           loop,
		chunkFrames:    chunkFrames,
		enqueueBytes:   enqueueFrames * bytesPerFrame,
		lookaheadBytes: lookaheadBytes,
		targetBytes:    targetBytes,
	})
}

func (cp *ChunkPlayer) StopStream() error {
	return cp.send(playerCmd{typ: cmdStopStream})
}

func (cp *ChunkPlayer) Tick() error {
	if cp == nil || !cp.inline {
		return nil
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.closed {
		return io.ErrClosedPipe
	}
	cp.serviceStreamLocked(true)
	return nil
}

func (cp *ChunkPlayer) SetBlockingPrefill(targetBytes int) {
	if cp == nil || cp.src == nil {
		return
	}
	cp.src.SetBlockingPrefill(targetBytes)
}

func (cp *ChunkPlayer) DisableBlockingPrefill() {
	if cp == nil || cp.src == nil {
		return
	}
	cp.src.DisableBlockingPrefill()
}

// EnqueueS16 sends interleaved stereo samples (s16) as a music chunk.
func (cp *ChunkPlayer) EnqueueS16(samples []int16) error {
	if cp == nil {
		return errors.New("music: nil chunk player")
	}
	if len(samples) == 0 {
		return nil
	}
	b := cp.src.acquireChunk(len(samples) * 2)
	b = PCMInt16ToBytesLEInto(b[:0], samples)
	return cp.send(playerCmd{typ: cmdEnqueue, data: b})
}

// EnqueueBytesS16LE sends little-endian signed 16-bit stereo bytes.
func (cp *ChunkPlayer) EnqueueBytesS16LE(chunk []byte) error {
	if cp == nil {
		return errors.New("music: nil chunk player")
	}
	if len(chunk) == 0 {
		return nil
	}
	b := cp.src.acquireChunk(len(chunk))
	copy(b, chunk)
	return cp.send(playerCmd{typ: cmdEnqueue, data: b})
}

func (cp *ChunkPlayer) Close() error {
	var err error
	cp.once.Do(func() {
		err = cp.send(playerCmd{typ: cmdClose})
		<-cp.done
	})
	return err
}

// BufferedBytes reports queued PCM bytes not yet consumed by audio reads.
func (cp *ChunkPlayer) BufferedBytes() int {
	if cp == nil || cp.src == nil {
		return 0
	}
	return cp.src.BufferedBytes()
}

func (cp *ChunkPlayer) send(cmd playerCmd) error {
	if cp == nil {
		return errors.New("music: nil chunk player")
	}
	if cp.inline {
		cp.mu.Lock()
		defer cp.mu.Unlock()
		if cp.closed {
			return io.ErrClosedPipe
		}
		if !cp.executeCommand(cmd) {
			cp.closed = true
			close(cp.done)
			if cmd.typ == cmdClose {
				return nil
			}
			return io.ErrClosedPipe
		}
		if cmd.done != nil {
			close(cmd.done)
		}
		return nil
	}
	select {
	case <-cp.done:
		return io.ErrClosedPipe
	default:
	}
	select {
	case cp.cmds <- cmd:
		return nil
	case <-cp.done:
		return io.ErrClosedPipe
	}
}

func (cp *ChunkPlayer) sendAndWait(cmd playerCmd) error {
	if err := cp.send(cmd); err != nil {
		return err
	}
	if cmd.done != nil {
		<-cmd.done
	}
	return nil
}

func (cp *ChunkPlayer) run() {
	ticker := time.NewTicker(12 * time.Millisecond)
	defer ticker.Stop()
	defer close(cp.done)
	for cmd := range cp.cmds {
		if !cp.executeCommand(cmd) {
			cp.closed = true
			return
		}
		if cmd.done != nil {
			close(cmd.done)
		}
		for cp.stream != nil {
			select {
			case cmd = <-cp.cmds:
				if !cp.executeCommand(cmd) {
					cp.closed = true
					return
				}
				if cmd.done != nil {
					close(cmd.done)
				}
			case <-ticker.C:
				cp.serviceStreamLocked(false)
				if cp.closed {
					return
				}
			}
		}
	}
}

func (cp *ChunkPlayer) executeCommand(cmd playerCmd) bool {
	switch cmd.typ {
	case cmdStart:
		cp.player.Play()
	case cmdStop:
		cp.clearStreamLocked()
		cp.player.Pause()
	case cmdClear:
		cp.clearStreamLocked()
		cp.src.Clear()
	case cmdReset:
		cp.clearStreamLocked()
		cp.src.Clear()
		cp.player.Pause()
		_ = cp.player.Close()
		p, err := cp.ctx.NewPlayer(cp.src)
		if err != nil {
			return false
		}
		cp.player = p
		cp.player.SetVolume(cp.volume)
	case cmdEnqueue:
		cp.src.Enqueue(cmd.data)
	case cmdSetVolume:
		cp.volume = cmd.vol
		cp.player.SetVolume(cmd.vol)
	case cmdSync:
	case cmdPlayStream:
		cp.startStreamLocked(cmd.streamFactory, cmd.loop, cmd.chunkFrames, cmd.enqueueBytes, cmd.lookaheadBytes, cmd.targetBytes)
	case cmdStopStream:
		cp.clearStreamLocked()
	case cmdClose:
		cp.clearStreamLocked()
		cp.player.Pause()
		cp.src.Close()
		_ = cp.player.Close()
		return false
	}
	return true
}

func (cp *ChunkPlayer) startStreamLocked(factory StreamFactory, loop bool, chunkFrames int, enqueueBytes int, lookaheadBytes int, targetBytes int) {
	cp.clearStreamLocked()
	if factory == nil {
		return
	}
	if chunkFrames <= 0 {
		chunkFrames = DefaultStreamChunkFrames()
	}
	if lookaheadBytes < 0 {
		lookaheadBytes = 0
	}
	if targetBytes < 0 {
		targetBytes = 0
	}
	cp.stream = &playerStream{
		factory:        factory,
		loop:           loop,
		chunkFrames:    chunkFrames,
		enqueueBytes:   enqueueBytes,
		lookaheadBytes: lookaheadBytes,
		targetBytes:    targetBytes,
	}
	cp.src.SetBlockingPrefill(targetBytes)
	cp.serviceStreamLocked(true)
}

func (cp *ChunkPlayer) clearStreamLocked() {
	cp.stream = nil
	cp.src.DisableBlockingPrefill()
}

func (cp *ChunkPlayer) serviceStreamLocked(fillFully bool) {
	for cp.stream != nil {
		sp := cp.stream
		buffered := cp.src.BufferedBytes()
		if !sp.started {
			for buffered < sp.targetBytes {
				chunk, alive := cp.nextStreamBatchLocked(sp, sp.targetBytes-buffered)
				if len(chunk) > 0 {
					cp.enqueueChunkLocked(chunk)
					buffered = cp.src.BufferedBytes()
				}
				if !alive || len(chunk) == 0 {
					break
				}
			}
			buffered = cp.src.BufferedBytes()
			if buffered == 0 && cp.stream == nil {
				return
			}
			if buffered >= sp.targetBytes || cp.stream == nil {
				cp.player.Play()
				sp.started = buffered > 0
			}
			if !fillFully {
				return
			}
			if cp.stream == nil {
				return
			}
		}
		buffered = cp.src.BufferedBytes()
		if buffered == 0 {
			cp.player.Pause()
			if cp.stream == nil {
				return
			}
			if cp.stream.renderer == nil && cp.stream.ended {
				cp.clearStreamLocked()
				return
			}
			cp.stream.started = false
			cp.src.SetBlockingPrefill(cp.stream.targetBytes)
			if !fillFully {
				return
			}
			continue
		}
		if buffered >= sp.lookaheadBytes && !fillFully {
			return
		}
		progress := false
		for buffered < sp.lookaheadBytes {
			chunk, alive := cp.nextStreamBatchLocked(sp, sp.lookaheadBytes-buffered)
			if len(chunk) > 0 {
				cp.enqueueChunkLocked(chunk)
				buffered = cp.src.BufferedBytes()
				progress = true
			}
			if !alive || len(chunk) == 0 {
				break
			}
			if !fillFully {
				return
			}
		}
		if !progress {
			return
		}
		if !fillFully {
			return
		}
	}
}

func (cp *ChunkPlayer) nextStreamBatchLocked(sp *playerStream, wantBytes int) ([]byte, bool) {
	if cp == nil || sp == nil {
		return nil, false
	}
	targetBytes := sp.enqueueBytes
	if targetBytes <= 0 {
		targetBytes = sp.chunkFrames * 4
	}
	if wantBytes > 0 && wantBytes < targetBytes {
		targetBytes = wantBytes
	}
	if targetBytes <= 0 {
		targetBytes = sp.chunkFrames * 4
	}
	var batch []byte
	alive := false
	for len(batch) < targetBytes {
		chunk, nextAlive := cp.nextStreamChunkLocked(sp)
		if len(chunk) > 0 {
			batch = append(batch, chunk...)
		}
		if nextAlive {
			alive = true
		}
		if !nextAlive || len(chunk) == 0 {
			break
		}
	}
	return batch, alive
}

func (cp *ChunkPlayer) nextStreamChunkLocked(sp *playerStream) ([]byte, bool) {
	if cp == nil || sp == nil || sp.factory == nil {
		return nil, false
	}
	if sp.ended {
		return nil, false
	}
	for {
		if sp.renderer == nil {
			next, err := sp.factory()
			if err != nil || next == nil {
				cp.clearStreamLocked()
				return nil, false
			}
			sp.renderer = next
		}
		chunk, done, err := sp.renderer.NextChunkS16LE(sp.chunkFrames)
		if err != nil {
			cp.clearStreamLocked()
			return nil, false
		}
		if done {
			sp.renderer = nil
			if !sp.loop {
				sp.ended = true
				return chunk, len(chunk) > 0
			}
			if len(chunk) > 0 {
				return chunk, true
			}
			continue
		}
		return chunk, true
	}
}

func (cp *ChunkPlayer) enqueueChunkLocked(chunk []byte) {
	if cp == nil || cp.src == nil || len(chunk) == 0 {
		return
	}
	b := cp.src.acquireChunk(len(chunk))
	copy(b, chunk)
	cp.src.Enqueue(b)
}

func startupPrefillBytes(lookaheadBytes int) int {
	if lookaheadBytes <= 0 {
		return 0
	}
	return lookaheadBytes
}

type pcmChunkBuffer struct {
	mu             sync.Mutex
	cond           *sync.Cond
	chunks         [][]byte
	chunkPool      sync.Pool
	off            int
	closed         bool
	bytes          int
	prefillBytes   int
	blockOnStarve  bool
	refillPending  bool
	starving       bool
	lastL          int16
	lastR          int16
	fadeOutPending bool
	fadeInPending  bool
}

func newPCMChunkBuffer() *pcmChunkBuffer {
	b := &pcmChunkBuffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *pcmChunkBuffer) acquireChunk(n int) []byte {
	if n <= 0 {
		return nil
	}
	if v := b.chunkPool.Get(); v != nil {
		chunk := v.([]byte)
		if cap(chunk) >= n {
			return chunk[:n]
		}
	}
	return make([]byte, n)
}

func (b *pcmChunkBuffer) releaseChunk(chunk []byte) {
	if chunk == nil {
		return
	}
	b.chunkPool.Put(chunk[:0])
}

func (b *pcmChunkBuffer) Enqueue(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	b.mu.Lock()
	if !b.closed {
		if b.starving || b.fadeInPending {
			applyFadeInLE(chunk, starvationFadeFrames())
			b.starving = false
			b.fadeInPending = false
		}
		b.chunks = append(b.chunks, chunk)
		b.bytes += len(chunk)
		if !b.blockOnStarve || b.bytes >= b.prefillBytes {
			b.refillPending = false
		}
		b.cond.Broadcast()
	}
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) Clear() {
	b.mu.Lock()
	for _, chunk := range b.chunks {
		b.releaseChunk(chunk)
	}
	b.chunks = b.chunks[:0]
	b.off = 0
	b.bytes = 0
	b.prefillBytes = 0
	b.blockOnStarve = false
	b.refillPending = false
	b.starving = false
	b.fadeOutPending = false
	b.fadeInPending = false
	b.lastL = 0
	b.lastR = 0
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) SetBlockingPrefill(targetBytes int) {
	b.mu.Lock()
	if targetBytes < 0 {
		targetBytes = 0
	}
	b.prefillBytes = targetBytes
	b.blockOnStarve = true
	b.refillPending = b.bytes < b.prefillBytes
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) DisableBlockingPrefill() {
	b.mu.Lock()
	b.prefillBytes = 0
	b.blockOnStarve = false
	b.refillPending = false
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) Close() {
	b.mu.Lock()
	b.closed = true
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for {
		if !b.closed && b.blockOnStarve {
			target := b.prefillBytes
			if target < 0 {
				target = 0
			}
			if b.refillPending || len(b.chunks) == 0 {
				if len(b.chunks) == 0 || b.bytes < target {
					b.refillPending = true
					b.cond.Wait()
					continue
				}
				b.refillPending = false
			}
		}
		if len(b.chunks) == 0 && !b.closed {
			n := b.fillStarvationAudio(p)
			if n > 0 {
				return n, nil
			}
		}
		if len(b.chunks) == 0 && b.closed {
			return 0, io.EOF
		}
		cur := b.chunks[0]
		if b.off >= len(cur) {
			b.releaseChunk(cur)
			b.chunks = b.chunks[1:]
			b.off = 0
			continue
		}
		n := copy(p, cur[b.off:])
		b.off += n
		b.bytes -= n
		if b.bytes < 0 {
			b.bytes = 0
		}
		if b.off >= len(cur) {
			b.releaseChunk(cur)
			b.chunks = b.chunks[1:]
			b.off = 0
		}
		b.captureLastSamples(p[:n])
		return n, nil
	}
}

func (b *pcmChunkBuffer) BufferedBytes() int {
	b.mu.Lock()
	n := b.bytes
	b.mu.Unlock()
	return n
}

func (b *pcmChunkBuffer) fillStarvationAudio(p []byte) int {
	if len(p) < 4 {
		return 0
	}
	frames := len(p) / 4
	if frames <= 0 {
		return 0
	}
	if !b.starving {
		b.starving = true
		b.fadeInPending = true
		if b.lastL != 0 || b.lastR != 0 {
			n := writeFadeOutLE(p[:frames*4], b.lastL, b.lastR, starvationFadeFrames())
			b.lastL = 0
			b.lastR = 0
			b.fadeOutPending = false
			return n
		}
	}
	clear(p[:frames*4])
	return frames * 4
}

func (b *pcmChunkBuffer) captureLastSamples(p []byte) {
	if len(p) < 4 {
		return
	}
	last := len(p) - 4
	b.lastL = int16(binary.LittleEndian.Uint16(p[last:]))
	b.lastR = int16(binary.LittleEndian.Uint16(p[last+2:]))
}

func starvationFadeFrames() int {
	return 128
}

func writeFadeOutLE(dst []byte, startL, startR int16, frames int) int {
	if frames <= 0 {
		return 0
	}
	avail := len(dst) / 4
	if avail <= 0 {
		return 0
	}
	if frames > avail {
		frames = avail
	}
	for i := 0; i < frames; i++ {
		remain := frames - i - 1
		l := int16((int32(startL) * int32(remain)) / int32(frames))
		r := int16((int32(startR) * int32(remain)) / int32(frames))
		binary.LittleEndian.PutUint16(dst[i*4:], uint16(l))
		binary.LittleEndian.PutUint16(dst[i*4+2:], uint16(r))
	}
	return frames * 4
}

func applyFadeInLE(chunk []byte, frames int) {
	if frames <= 0 {
		return
	}
	avail := len(chunk) / 4
	if avail <= 0 {
		return
	}
	if frames > avail {
		frames = avail
	}
	for i := 0; i < frames; i++ {
		scaleNum := i + 1
		l := int16(binary.LittleEndian.Uint16(chunk[i*4:]))
		r := int16(binary.LittleEndian.Uint16(chunk[i*4+2:]))
		l = int16((int32(l) * int32(scaleNum)) / int32(frames))
		r = int16((int32(r) * int32(scaleNum)) / int32(frames))
		binary.LittleEndian.PutUint16(chunk[i*4:], uint16(l))
		binary.LittleEndian.PutUint16(chunk[i*4+2:], uint16(r))
	}
}
