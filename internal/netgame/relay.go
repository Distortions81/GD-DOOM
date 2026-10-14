package netgame

import (
	"context"
	"time"
)

// RelayMessages forwards bounded binary messages between a server-side
// transport and a worker connection. It does not decode or recompress world
// states, invent acknowledgments, or change the authoritative session.
func RelayMessages(ctx context.Context, client, worker MessageTransport) {
	stop := context.AfterFunc(ctx, func() { client.Close(); worker.Close() })
	defer stop()
	defer client.Close()
	defer worker.Close()
	pump := func(dst, src MessageTransport) {
		for {
			if deadlines, ok := src.(interface{ SetReadDeadline(time.Time) error }); ok {
				_ = deadlines.SetReadDeadline(time.Now().Add(10 * time.Second))
			}
			message, err := src.ReadMessage()
			if err != nil || dst.WriteMessage(message) != nil {
				return
			}
		}
	}
	done := make(chan struct{})
	go func() { defer close(done); defer worker.Close(); pump(worker, client) }()
	pump(client, worker)
	client.Close()
	worker.Close()
	<-done
}
