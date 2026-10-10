package netgame

import "net"

// DatagramTransport still delivers reliable control messages in order. Its
// snapshots may be lost/reordered, so clients recover a missing baseline and
// discard superseded epoch traffic instead of treating it as a stream error.
type DatagramTransport interface {
	MessageTransport
	UnreliableSnapshots() bool
}

func readPeerClientMessage(conn net.Conn) (any, error) {
	if transport, ok := conn.(MessageTransport); ok {
		return transport.ReadMessage()
	}
	return ReadClientMessage(conn)
}
