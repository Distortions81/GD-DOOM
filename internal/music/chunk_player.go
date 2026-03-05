package music

import (
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
	cmdEnqueue
	cmdSetVolume
	cmdClose
)

type playerCmd struct {
	typ  playerCmdType
	data []byte
	vol  float64
}

// ChunkPlayer plays queued 44.1kHz s16 stereo chunks on a dedicated goroutine.
type ChunkPlayer struct {
	ctx    *audio.Context
	player *audio.Player
	src    *pcmChunkBuffer

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
	p, err := audio.NewPlayer(ctx, src)
	if err != nil {
		return nil, err
	}
	cp := &ChunkPlayer{
		ctx:    ctx,
		player: p,
		src:    src,
		cmds:   make(chan playerCmd, 64),
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

// EnqueueS16 sends interleaved stereo samples (s16) as a music chunk.
func (cp *ChunkPlayer) EnqueueS16(samples []int16) error {
	return cp.EnqueueBytesS16LE(PCMInt16ToBytesLE(samples))
}

// EnqueueBytesS16LE sends little-endian signed 16-bit stereo bytes.
func (cp *ChunkPlayer) EnqueueBytesS16LE(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	b := make([]byte, len(chunk))
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

func (cp *ChunkPlayer) run() {
	defer close(cp.done)
	playing := false
	for cmd := range cp.cmds {
		switch cmd.typ {
		case cmdStart:
			if !playing {
				cp.player.Play()
				playing = true
			}
		case cmdStop:
			cp.player.Pause()
			playing = false
		case cmdClear:
			cp.src.Clear()
		case cmdEnqueue:
			cp.src.Enqueue(cmd.data)
		case cmdSetVolume:
			cp.player.SetVolume(cmd.vol)
		case cmdClose:
			cp.player.Pause()
			cp.src.Close()
			_ = cp.player.Close()
			return
		}
	}
}

type pcmChunkBuffer struct {
	mu     sync.Mutex
	cond   *sync.Cond
	chunks [][]byte
	off    int
	closed bool
}

func newPCMChunkBuffer() *pcmChunkBuffer {
	b := &pcmChunkBuffer{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *pcmChunkBuffer) Enqueue(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	b.mu.Lock()
	if !b.closed {
		b.chunks = append(b.chunks, chunk)
		b.cond.Signal()
	}
	b.mu.Unlock()
}

func (b *pcmChunkBuffer) Clear() {
	b.mu.Lock()
	b.chunks = b.chunks[:0]
	b.off = 0
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
	for len(b.chunks) == 0 && !b.closed {
		b.cond.Wait()
	}
	if len(b.chunks) == 0 && b.closed {
		return 0, io.EOF
	}
	cur := b.chunks[0]
	if b.off >= len(cur) {
		b.chunks = b.chunks[1:]
		b.off = 0
		return b.Read(p)
	}
	n := copy(p, cur[b.off:])
	b.off += n
	if b.off >= len(cur) {
		b.chunks = b.chunks[1:]
		b.off = 0
	}
	return n, nil
}
