//go:build js && wasm

package netgame

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall/js"
	"time"
)

// OpenWebTransport uses the browser's authenticated HTTP/3 transport. There is
// no certificate-validation bypass or origin-independent network fallback.
func OpenWebTransport(ctx context.Context, rawURL string) (MessageTransport, error) {
	return openBrowserWebTransport(ctx, rawURL, js.Undefined())
}

// The options parameter is internal so browser integration tests can explicitly
// pin their own short-lived certificate with the standard WebTransport API.
// Application connections always use the browser's ordinary trust store.
func openBrowserWebTransport(ctx context.Context, rawURL string, options js.Value) (MessageTransport, error) {
	url, err := webTransportURL(rawURL)
	if err != nil {
		return nil, err
	}
	constructor := js.Global().Get("WebTransport")
	if constructor.Type() != js.TypeFunction {
		return nil, errors.New("WebTransport is unavailable in this browser or secure context")
	}
	transport, err := wtJSValue(func() js.Value {
		if options.IsUndefined() {
			return constructor.New(url)
		}
		return constructor.New(url, options)
	})
	if err != nil {
		return nil, fmt.Errorf("create browser WebTransport: %w", err)
	}
	owned, cancel := context.WithCancel(ctx)
	opening, stopOpening := context.WithTimeout(owned, 5*time.Second)
	defer stopOpening()
	session := &browserWTSession{ctx: owned, cancel: cancel, transport: transport}
	context.AfterFunc(owned, func() { _ = session.Close() })
	// Always observe closed, including a failed handshake, so the browser does
	// not retain an unhandled rejected Promise and all pending reads wake up.
	go func() {
		_, closedErr := awaitWTPromise(context.Background(), transport.Get("closed"))
		session.errMu.Lock()
		session.closedErr = closedErr
		session.errMu.Unlock()
		cancel()
	}()
	fail := func(err error) (MessageTransport, error) { session.Close(); return nil, err }
	if _, err := awaitWTPromise(opening, transport.Get("ready")); err != nil {
		session.errMu.Lock()
		closedErr := session.closedErr
		session.errMu.Unlock()
		if closedErr != nil {
			err = closedErr
		}
		return fail(fmt.Errorf("browser WebTransport handshake: %w", err))
	}
	// Bound the browser's own queues as well as the Go receive queue. These
	// settings concern time spent queued locally, not time spent on the wire.
	datagrams := transport.Get("datagrams")
	for _, setting := range []struct {
		name  string
		value int
	}{
		{"incomingMaxAge", 100}, {"outgoingMaxAge", 100},
		{"incomingMaxBufferedDatagrams", 4}, {"outgoingMaxBufferedDatagrams", 1},
		{"incomingHighWaterMark", 4}, {"outgoingHighWaterMark", 1},
	} {
		if !datagrams.Get(setting.name).IsUndefined() {
			if _, err := wtJSValue(func() js.Value { datagrams.Set(setting.name, setting.value); return js.Undefined() }); err != nil {
				return fail(err)
			}
		}
	}
	streamPromise, err := wtJSValue(func() js.Value { return transport.Call("createBidirectionalStream") })
	if err != nil {
		return fail(err)
	}
	stream, err := awaitWTPromise(opening, streamPromise)
	if err != nil {
		return fail(err)
	}
	reader, err := wtJSValue(func() js.Value { return stream.Get("readable").Call("getReader") })
	if err != nil {
		return fail(err)
	}
	writer, err := wtJSValue(func() js.Value { return stream.Get("writable").Call("getWriter") })
	if err != nil {
		return fail(err)
	}
	session.datagramReader, err = wtJSValue(func() js.Value { return transport.Get("datagrams").Get("readable").Call("getReader") })
	if err != nil {
		return fail(err)
	}
	session.datagramWriter, err = wtJSValue(func() js.Value { return transport.Get("datagrams").Get("writable").Call("getWriter") })
	if err != nil {
		return fail(err)
	}
	conn := &browserWTConn{session: session, reader: reader, writer: writer, chunks: make(chan browserWTChunk, 1), deadlineChanged: make(chan struct{}), address: browserWTAddr(url)}
	go conn.readPump()
	return newWebTransportTransport(owned, conn, session, false), nil
}

type browserWTSession struct {
	ctx                                       context.Context
	cancel                                    context.CancelFunc
	transport, datagramReader, datagramWriter js.Value
	readMu, writeMu                           sync.Mutex
	closeOnce                                 sync.Once
	errMu                                     sync.Mutex
	closedErr                                 error
	datagramPending                           bool
}

func (s *browserWTSession) Close() error {
	s.closeOnce.Do(func() { s.cancel(); _, _ = wtJSValue(func() js.Value { return s.transport.Call("close") }) })
	return nil
}

func (s *browserWTSession) SendDatagram(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.datagramPending {
		return nil // Treat browser backpressure as loss; future bundles repair it.
	}
	maximum := s.transport.Get("datagrams").Get("maxDatagramSize")
	if maximum.Type() == js.TypeNumber && len(data) > maximum.Int() {
		return nil
	}
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	promise, err := wtJSValue(func() js.Value { return s.datagramWriter.Call("write", bytes) })
	if err != nil {
		return err
	}
	s.datagramPending = true
	// The browser may defer a write under congestion. Never hold the client's
	// control writer behind it; retain at most one outstanding datagram Promise.
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		_, err := awaitWTPromise(ctx, promise)
		s.writeMu.Lock()
		s.datagramPending = false
		s.writeMu.Unlock()
		if err != nil {
			s.Close()
		}
	}()
	return nil
}

func (s *browserWTSession) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	promise, err := wtJSValue(func() js.Value { return s.datagramReader.Call("read") })
	if err != nil {
		return nil, err
	}
	// This is the common adapter's lifetime context. A cancelled receive pump
	// leaves one pending Promise until its graceful stream drain ends and closes
	// the session. Closing here would discard queued reliable terminal messages.
	result, err := awaitWTPromise(ctx, promise)
	if err != nil {
		return nil, err
	}
	if result.Get("done").Bool() {
		return nil, io.EOF
	}
	return wtCopyBytes(result.Get("value"), 64<<10)
}

type browserWTChunk struct {
	data []byte
	err  error
}
type browserWTAddr string

func (a browserWTAddr) Network() string { return "webtransport" }
func (a browserWTAddr) String() string  { return string(a) }

type browserWTConn struct {
	session                     *browserWTSession
	reader, writer              js.Value
	chunks                      chan browserWTChunk
	buffer                      []byte
	readMu, writeMu, mu         sync.Mutex
	readDeadline, writeDeadline time.Time
	deadlineChanged             chan struct{}
	writeClosed                 bool
	address                     browserWTAddr
}

func (c *browserWTConn) readPump() {
	defer close(c.chunks)
	for {
		promise, err := wtJSValue(func() js.Value { return c.reader.Call("read") })
		var data []byte
		if err == nil {
			var result js.Value
			result, err = awaitWTPromise(c.session.ctx, promise)
			if err == nil {
				if result.Get("done").Bool() {
					err = io.EOF
				} else {
					data, err = wtCopyBytes(result.Get("value"), MaxSnapshotBytes+snapshotBodyHeaderBytes+messageHeaderBytes)
				}
			}
		}
		if err == nil && len(data) == 0 {
			continue
		}
		select {
		case c.chunks <- browserWTChunk{data, err}:
		case <-c.session.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *browserWTConn) Read(dst []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(dst) == 0 {
		return 0, nil
	}
	for len(c.buffer) == 0 {
		deadline, changed := c.deadline(false)
		timer, timeout := wtDeadlineTimer(deadline)
		var chunk browserWTChunk
		var ok bool
		select {
		case chunk, ok = <-c.chunks:
			if timer != nil {
				timer.Stop()
			}
			if !ok {
				return 0, io.EOF
			}
			if chunk.err != nil {
				return 0, chunk.err
			}
			c.buffer = chunk.data
		case <-c.session.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return 0, net.ErrClosed
		case <-timeout:
			return 0, os.ErrDeadlineExceeded
		case <-changed:
			if timer != nil {
				timer.Stop()
			}
			continue
		}
	}
	n := copy(dst, c.buffer)
	c.buffer = c.buffer[n:]
	return n, nil
}

func (c *browserWTConn) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.mu.Lock()
	closed := c.writeClosed
	c.mu.Unlock()
	if closed {
		return 0, net.ErrClosed
	}
	if err := c.session.ctx.Err(); err != nil {
		return 0, net.ErrClosed
	}
	if len(data) == 0 {
		return 0, nil
	}
	deadline, _ := c.deadline(true)
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return 0, os.ErrDeadlineExceeded
	}
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	promise, err := wtJSValue(func() js.Value { return c.writer.Call("write", bytes) })
	if err != nil {
		return 0, err
	}
	result, err := wtPromiseResult(promise)
	if err != nil {
		return 0, err
	}
	for {
		deadline, changed := c.deadline(true)
		timer, timeout := wtDeadlineTimer(deadline)
		select {
		case response := <-result:
			if timer != nil {
				timer.Stop()
			}
			if response.err != nil {
				return 0, response.err
			}
			return len(data), nil
		case <-c.session.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return 0, net.ErrClosed
		case <-timeout:
			c.session.Close()
			return 0, os.ErrDeadlineExceeded
		case <-changed:
			if timer != nil {
				timer.Stop()
			}
			continue
		}
	}
}

// Close sends FIN on the reliable write side; the common adapter drains the
// peer's remaining messages before it calls session.Close for a hard shutdown.
func (c *browserWTConn) Close() error {
	c.mu.Lock()
	if c.writeClosed {
		c.mu.Unlock()
		return nil
	}
	c.writeClosed = true
	c.mu.Unlock()
	promise, err := wtJSValue(func() js.Value { return c.writer.Call("close") })
	if err != nil {
		return err
	}
	_, err = wtPromiseResult(promise)
	return err
}
func (c *browserWTConn) LocalAddr() net.Addr                { return browserWTAddr("browser") }
func (c *browserWTConn) RemoteAddr() net.Addr               { return c.address }
func (c *browserWTConn) SetDeadline(t time.Time) error      { return c.setDeadline(t, true, true) }
func (c *browserWTConn) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, true, false) }
func (c *browserWTConn) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, false, true) }
func (c *browserWTConn) setDeadline(t time.Time, read, write bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if read {
		c.readDeadline = t
	}
	if write {
		c.writeDeadline = t
	}
	close(c.deadlineChanged)
	c.deadlineChanged = make(chan struct{})
	return nil
}
func (c *browserWTConn) deadline(write bool) (time.Time, <-chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if write {
		return c.writeDeadline, c.deadlineChanged
	}
	return c.readDeadline, c.deadlineChanged
}
func wtDeadlineTimer(deadline time.Time) (*time.Timer, <-chan time.Time) {
	if deadline.IsZero() {
		return nil, nil
	}
	timer := time.NewTimer(time.Until(deadline))
	return timer, timer.C
}

type browserWTResult struct {
	value js.Value
	err   error
}

func wtPromiseResult(promise js.Value) (<-chan browserWTResult, error) {
	result := make(chan browserWTResult, 1)
	settled := make(chan struct{})
	resolve := js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		result <- browserWTResult{value: value}
		close(settled)
		return nil
	})
	reject := js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := "browser WebTransport operation failed"
		if len(args) > 0 {
			if args[0].Type() == js.TypeObject && !args[0].IsNull() && args[0].Get("message").Type() == js.TypeString {
				message = args[0].Get("message").String()
			} else {
				message = args[0].String()
			}
		}
		result <- browserWTResult{err: errors.New(message)}
		close(settled)
		return nil
	})
	if _, err := wtJSValue(func() js.Value { return promise.Call("then", resolve, reject) }); err != nil {
		resolve.Release()
		reject.Release()
		return nil, err
	}
	// Keep callbacks until settlement even if an awaiting context is cancelled.
	// Session.Close makes outstanding stream/ready promises settle, releasing them.
	go func() { <-settled; resolve.Release(); reject.Release() }()
	return result, nil
}
func awaitWTPromise(ctx context.Context, promise js.Value) (js.Value, error) {
	result, err := wtPromiseResult(promise)
	if err != nil {
		return js.Undefined(), err
	}
	select {
	case response := <-result:
		return response.value, response.err
	case <-ctx.Done():
		return js.Undefined(), ctx.Err()
	}
}
func wtJSValue(call func() js.Value) (value js.Value, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("browser WebTransport: %v", failure)
		}
	}()
	return call(), nil
}
func wtCopyBytes(value js.Value, limit int) (data []byte, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("invalid browser WebTransport bytes: %v", failure)
		}
	}()
	size := value.Get("byteLength").Int()
	if size < 0 || size > limit {
		return nil, ErrProtocol
	}
	data = make([]byte, size)
	if js.CopyBytesToGo(data, value) != size {
		return nil, ErrProtocol
	}
	return data, nil
}
