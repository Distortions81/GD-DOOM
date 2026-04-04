package netplay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"gddoom/internal/demo"
)

const protocolVersion = 1

type SessionConfig struct {
	WADHash          string `json:"wad_hash"`
	MapName          string `json:"map"`
	PlayerSlot       int    `json:"player_slot"`
	SkillLevel       int    `json:"skill_level"`
	GameMode         string `json:"game_mode"`
	ShowNoSkillItems bool   `json:"show_no_skill_items"`
	ShowAllItems     bool   `json:"show_all_items"`
	FastMonsters     bool   `json:"fast_monsters"`
	RespawnMonsters  bool   `json:"respawn_monsters"`
	NoMonsters       bool   `json:"no_monsters"`
	AutoWeaponSwitch bool   `json:"auto_weapon_switch"`
	CheatLevel       int    `json:"cheat_level"`
	Invulnerable     bool   `json:"invulnerable"`
	SourcePortMode   bool   `json:"source_port_mode"`
}

type packet struct {
	Type     string         `json:"type"`
	Protocol int            `json:"protocol,omitempty"`
	Session  *SessionConfig `json:"session,omitempty"`
	Tic      *demo.Tic      `json:"tic,omitempty"`
}

type broadcasterClient struct {
	conn net.Conn
	enc  *json.Encoder
}

type Broadcaster struct {
	listener net.Listener
	session  SessionConfig

	mu         sync.Mutex
	clients    map[int]broadcasterClient
	nextClient int

	firstViewer chan struct{}
	firstOnce   sync.Once
	closeOnce   sync.Once
	closed      chan struct{}
	wg          sync.WaitGroup
}

func Listen(addr string, session SessionConfig) (*Broadcaster, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("broadcast address is required")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	b := &Broadcaster{
		listener:    ln,
		session:     session,
		clients:     make(map[int]broadcasterClient),
		firstViewer: make(chan struct{}),
		closed:      make(chan struct{}),
	}
	b.wg.Add(1)
	go b.acceptLoop(ln)
	return b, nil
}

func (b *Broadcaster) Addr() string {
	if b == nil || b.listener == nil {
		return ""
	}
	return b.listener.Addr().String()
}

func (b *Broadcaster) WaitForViewer(ctx context.Context) error {
	if b == nil {
		return fmt.Errorf("broadcaster is nil")
	}
	select {
	case <-b.firstViewer:
		return nil
	case <-b.closed:
		return io.ErrClosedPipe
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Broadcaster) StopAccepting() error {
	if b == nil || b.listener == nil {
		return nil
	}
	err := b.listener.Close()
	b.listener = nil
	return err
}

func (b *Broadcaster) BroadcastTic(tc demo.Tic) error {
	if b == nil {
		return nil
	}
	msg := packet{Type: "tic", Tic: &tc}
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, client := range b.clients {
		if err := client.enc.Encode(msg); err != nil {
			_ = client.conn.Close()
			delete(b.clients, id)
		}
	}
	return nil
}

func (b *Broadcaster) Close() error {
	if b == nil {
		return nil
	}
	var err error
	b.closeOnce.Do(func() {
		close(b.closed)
		if b.listener != nil {
			err = b.listener.Close()
			b.listener = nil
		}
		b.mu.Lock()
		for id, client := range b.clients {
			_ = client.conn.Close()
			delete(b.clients, id)
		}
		b.mu.Unlock()
		b.wg.Wait()
	})
	return err
}

func (b *Broadcaster) acceptLoop(ln net.Listener) {
	defer b.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			select {
			case <-b.closed:
				return
			default:
				continue
			}
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
		client := broadcasterClient{conn: conn, enc: json.NewEncoder(conn)}
		if err := client.enc.Encode(packet{
			Type:     "hello",
			Protocol: protocolVersion,
			Session:  &b.session,
		}); err != nil {
			_ = conn.Close()
			continue
		}
		b.mu.Lock()
		id := b.nextClient
		b.nextClient++
		b.clients[id] = client
		b.mu.Unlock()
		b.firstOnce.Do(func() { close(b.firstViewer) })
	}
}

type Viewer struct {
	conn    net.Conn
	session SessionConfig
	tics    chan demo.Tic

	mu     sync.Mutex
	err    error
	closed bool
	wg     sync.WaitGroup
}

func Dial(addr string, localWADHash string) (*Viewer, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("watch address is required")
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	dec := json.NewDecoder(conn)
	var hello packet
	if err := dec.Decode(&hello); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read broadcast hello: %w", err)
	}
	if hello.Type != "hello" {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected broadcast packet %q", hello.Type)
	}
	if hello.Protocol != protocolVersion {
		_ = conn.Close()
		return nil, fmt.Errorf("unsupported broadcast protocol %d", hello.Protocol)
	}
	if hello.Session == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("broadcast hello missing session config")
	}
	if localWADHash != "" && hello.Session.WADHash != "" && hello.Session.WADHash != localWADHash {
		_ = conn.Close()
		return nil, fmt.Errorf("broadcast WAD hash mismatch: local=%s host=%s", localWADHash, hello.Session.WADHash)
	}
	v := &Viewer{
		conn:    conn,
		session: *hello.Session,
		tics:    make(chan demo.Tic, 256),
	}
	v.wg.Add(1)
	go v.readLoop(dec)
	return v, nil
}

func (v *Viewer) Session() SessionConfig {
	if v == nil {
		return SessionConfig{}
	}
	return v.session
}

func (v *Viewer) PollTic() (demo.Tic, bool, error) {
	if v == nil {
		return demo.Tic{}, false, nil
	}
	select {
	case tc, ok := <-v.tics:
		if ok {
			return tc, true, nil
		}
		return demo.Tic{}, false, v.readErr()
	default:
		return demo.Tic{}, false, v.readErr()
	}
}

func (v *Viewer) Close() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	v.mu.Unlock()
	err := v.conn.Close()
	v.wg.Wait()
	return err
}

func (v *Viewer) readLoop(dec *json.Decoder) {
	defer v.wg.Done()
	defer close(v.tics)
	for {
		var msg packet
		if err := dec.Decode(&msg); err != nil {
			v.setErr(err)
			return
		}
		if msg.Type != "tic" || msg.Tic == nil {
			v.setErr(fmt.Errorf("unexpected broadcast packet %q", msg.Type))
			return
		}
		v.tics <- *msg.Tic
	}
}

func (v *Viewer) readErr() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}

func (v *Viewer) setErr(err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err == nil {
		v.err = err
	}
}
