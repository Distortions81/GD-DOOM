package roomhost

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/lobby"
)

func TestWorkerReadinessRequiresExpectedContentAndPrivateEndpoints(t *testing.T) {
	pack := testPacks(t)[0]
	manifest, err := lobby.ValidateSettings(testRequest(t, "Ready", "E1M1", "coop").Settings, pack.Pack)
	if err != nil {
		t.Fatal(err)
	}
	ready := WorkerReady{Version: 1, TCPAddress: "127.0.0.1:1234", WebURL: "http://127.0.0.1:1235/netplay", Manifest: manifest}
	filename := filepath.Join(t.TempDir(), "ready.json")
	if err := WriteWorkerReady(filename, ready); err != nil {
		t.Fatal(err)
	}
	read, err := snapshotReadyFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateWorkerReady(read, manifest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filename)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("worker endpoints must be private")
	}
	for _, mutate := range []func(*WorkerReady){
		func(r *WorkerReady) { r.Manifest.Skill++ },
		func(r *WorkerReady) { r.TCPAddress = "192.0.2.1:1234" },
		func(r *WorkerReady) { r.WebURL = "http://localhost:1235/netplay" },
		func(r *WorkerReady) { r.WebURL = "http://127.0.0.1:1235/netplay?other=1" },
		func(r *WorkerReady) { r.WebURL = "http://user@127.0.0.1:1235/netplay" },
		func(r *WorkerReady) { r.Version++ },
	} {
		candidate := ready
		mutate(&candidate)
		if _, err := validateWorkerReady(candidate, manifest); err == nil {
			t.Fatalf("accepted invalid worker readiness: %+v", candidate)
		}
	}
}
