package roomhost

import (
	"encoding/json"
	"os"

	"gddoom/internal/netgame"
)

// WorkerReady names bound private endpoints. The supervisor additionally makes
// a protocol status request before advertising the worker as ready.
type WorkerReady struct {
	Version    int                           `json:"version"`
	TCPAddress string                        `json:"tcp_address"`
	WebURL     string                        `json:"web_url"`
	Manifest   netgame.CompatibilityManifest `json:"manifest"`
}

func WriteWorkerReady(filename string, ready WorkerReady) error {
	f, err := os.OpenFile(filename+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(filename + ".tmp")
	err = json.NewEncoder(f).Encode(ready)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(filename+".tmp", filename)
}
