package music

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"

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
	cmdClose
)

type playerCmd struct {
	typ  playerCmdType
	data []byte
	vol  float64
	done chan struct{}
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
	}
	go cp.run()
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
	defer close(cp.done)
	for cmd := range cp.cmds {
		if !cp.executeCommand(cmd) {
			return
		}
		if cmd.done != nil {
			close(cmd.done)
		}
	}
}

func (cp *ChunkPlayer) executeCommand(cmd playerCmd) bool {
	switch cmd.typ {
	case cmdStart:
		cp.player.Play()
	case cmdStop:
		cp.player.Pause()
	case cmdClear:
		cp.src.Clear()
	case cmdReset:
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
	case cmdClose:
		cp.player.Pause()
		cp.src.Close()
		_ = cp.player.Close()
		return false
	}
	return true
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
