package netgame

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sync"
	"time"
)

const MaxGameDatagramBytes = 1100
const webTransportDrainTimeout = 2 * time.Second

type webTransportSession interface {
	SendDatagram([]byte) error
	ReceiveDatagram(context.Context) ([]byte, error)
	Close() error
}

type transportRead struct {
	message any
	err     error
}

// webTransportTransport shares application framing between native QUIC and the
// browser API. Control and full states keep reliable order; datagrams have an
// independent bounded queue and may be discarded before they reach simulation.
type webTransportTransport struct {
	stream       net.Conn
	session      webTransportSession
	serverSide   bool
	reliable     chan transportRead
	datagrams    chan any
	failure      chan error
	done         chan struct{}
	closed       chan struct{}
	reliableDone chan struct{}
	cancel       context.CancelFunc
	once         sync.Once
	writeMu      sync.Mutex
	deadlineMu   sync.Mutex
	readDeadline time.Time
	readChanged  chan struct{}
}

func webTransportURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "wt" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("WebTransport requires an HTTPS URL without credentials or fragment")
	}
	if parsed.Path == "" {
		parsed.Path = "/netplay"
	}
	return parsed.String(), nil
}

func DialWebTransport(ctx context.Context, address string, hello Hello) (*Client, error) {
	transport, err := OpenWebTransport(ctx, address)
	if err != nil {
		return nil, err
	}
	return Connect(ctx, transport, hello)
}

func newWebTransportTransport(ctx context.Context, stream net.Conn, session webTransportSession, serverSide bool) *webTransportTransport {
	lifetime, cancel := context.WithCancel(ctx)
	transport := &webTransportTransport{stream: stream, session: session, serverSide: serverSide, reliable: make(chan transportRead, 1), datagrams: make(chan any, 32), failure: make(chan error, 1), done: make(chan struct{}), closed: make(chan struct{}), reliableDone: make(chan struct{}), cancel: cancel, readChanged: make(chan struct{})}
	_ = transport.SetDeadline(time.Now().Add(5 * time.Second))
	go transport.readReliable()
	go transport.readDatagrams(lifetime)
	stop := context.AfterFunc(ctx, func() { _ = transport.Close() })
	go func() { <-transport.closed; stop() }()
	return transport
}

func (t *webTransportTransport) readReliable() {
	defer close(t.reliableDone)
	for {
		var message any
		var err error
		if t.serverSide {
			message, err = ReadClientMessage(t.stream)
		} else {
			message, err = ReadMessage(t.stream)
		}
		select {
		case t.reliable <- transportRead{message: message, err: err}:
		case <-t.done:
			// During graceful close, drain only until the peer's FIN. Its receipt of
			// our FIN proves all earlier control/snapshot bytes reached the peer.
		}
		if err != nil {
			return
		}
	}
}

func decodeGameDatagram(data []byte, serverSide bool) (any, error) {
	if len(data) > MaxGameDatagramBytes {
		return nil, ErrProtocol
	}
	reader := bytes.NewReader(data)
	message, err := readMessage(reader, serverSide)
	if err != nil || reader.Len() != 0 {
		return nil, ErrProtocol
	}
	if serverSide {
		if _, ok := message.(InputBatch); !ok {
			return nil, ErrProtocol
		}
	} else {
		snapshot, ok := message.(Snapshot)
		if !ok || snapshot.Encoding != SnapshotDeltaZstd || snapshot.BaselineID == 0 {
			return nil, ErrProtocol
		}
	}
	return message, nil
}

func (t *webTransportTransport) readDatagrams(ctx context.Context) {
	for {
		data, err := t.session.ReceiveDatagram(ctx)
		if err != nil {
			return
		} // Reliable stream/session closure carries terminal status.
		message, err := decodeGameDatagram(data, t.serverSide)
		if err != nil {
			select {
			case t.failure <- err:
			default:
			}
			return
		}
		select {
		case t.datagrams <- message:
		default:
		} // Reliable control can never be evicted.
	}
}

func (t *webTransportTransport) ReadMessage() (any, error) {
	for {
		// Process already-arrived epoch/terminal controls before replaceable traffic.
		select {
		case read := <-t.reliable:
			return read.message, read.err
		default:
		}
		t.deadlineMu.Lock()
		deadline, changed := t.readDeadline, t.readChanged
		t.deadlineMu.Unlock()
		var timer *time.Timer
		var timeout <-chan time.Time
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil, os.ErrDeadlineExceeded
			}
			timer = time.NewTimer(remaining)
			timeout = timer.C
		}
		var read transportRead
		var retry bool
		select {
		case read = <-t.reliable:
		case message := <-t.datagrams:
			read.message = message
		case err := <-t.failure:
			read.err = err
		case <-t.done:
			read.err = net.ErrClosed
		case <-timeout:
			read.err = os.ErrDeadlineExceeded
		case <-changed:
			retry = true
		}
		if timer != nil {
			timer.Stop()
		}
		if retry {
			continue
		}
		return read.message, read.err
	}
}

func gameMessageDatagram(message any, size int) bool {
	if size > MaxGameDatagramBytes {
		return false
	}
	switch m := message.(type) {
	case InputBatch:
		return len(m.Inputs) != 0 || m.SnapshotAck != 0 // Recovery requests stay reliable.
	case Snapshot:
		return m.Encoding == SnapshotDeltaZstd && m.BaselineID != 0
	default:
		return false
	}
}

func (t *webTransportTransport) WriteMessage(message any) error {
	data, err := MarshalMessage(message)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	select {
	case <-t.done:
		return net.ErrClosed
	default:
	}
	if gameMessageDatagram(message, len(data)) {
		// A congestion/MTU refusal is equivalent to datagram loss; do not put an
		// expired input behind a large reliable snapshot. Future bundles repair it.
		return t.session.SendDatagram(data)
	}
	_ = t.stream.SetWriteDeadline(time.Now().Add(2 * time.Second))
	for len(data) > 0 {
		n, err := t.stream.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func (*webTransportTransport) UnreliableSnapshots() bool        { return true }
func (t *webTransportTransport) Read(data []byte) (int, error)  { return t.stream.Read(data) }
func (t *webTransportTransport) Write(data []byte) (int, error) { return t.stream.Write(data) }
func (t *webTransportTransport) LocalAddr() net.Addr            { return t.stream.LocalAddr() }
func (t *webTransportTransport) RemoteAddr() net.Addr           { return t.stream.RemoteAddr() }
func (t *webTransportTransport) SetReadDeadline(deadline time.Time) error {
	t.deadlineMu.Lock()
	t.readDeadline = deadline
	close(t.readChanged)
	t.readChanged = make(chan struct{})
	t.deadlineMu.Unlock()
	// The stream pump has no idle deadline: datagrams can legitimately carry all
	// current activity while the reliable stream is idle.
	return nil
}
func (t *webTransportTransport) SetWriteDeadline(deadline time.Time) error {
	return t.stream.SetWriteDeadline(deadline)
}
func (t *webTransportTransport) SetDeadline(deadline time.Time) error {
	return errors.Join(t.SetReadDeadline(deadline), t.SetWriteDeadline(deadline))
}
func (t *webTransportTransport) Close() error {
	t.once.Do(func() {
		close(t.done)
		t.cancel()
		// Half-close first: closing the QUIC session immediately resets queued
		// streams and can discard a successful query/final snapshot write.
		go func() {
			defer close(t.closed)
			_ = t.stream.Close()
			timer := time.NewTimer(webTransportDrainTimeout)
			defer timer.Stop()
			select {
			case <-t.reliableDone:
			case <-timer.C:
			}
			_ = t.stream.SetReadDeadline(time.Now())
			_ = t.session.Close()
		}()
	})
	return nil
}

func (t *webTransportTransport) waitClosed() { <-t.closed }
