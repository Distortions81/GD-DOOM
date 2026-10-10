//go:build !js

package netgame

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"

	wt "github.com/quic-go/webtransport-go"
)

func webTransportTestCertificate(t *testing.T) (*tls.Config, *tls.Config, [32]byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(10 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key, Leaf: certificate}}}, &tls.Config{RootCAs: roots}, sha256.Sum256(der)
}

type nativeWebTransportFixture struct {
	server  *Server
	address string
	trust   *tls.Config
	pin     [32]byte
	ctx     context.Context
	match   *Match
}

func startNativeWebTransportFixture(t *testing.T, match *Match, options WebTransportOptions) nativeWebTransportFixture {
	t.Helper()
	server, err := NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	tlsServer, tlsClient, pin := webTransportTestCertificate(t)
	webServer, err := server.NewWebTransportServer(tlsServer, options)
	if err != nil {
		t.Fatal(err)
	}
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := &readyListener{Listener: newPipeListener(), ready: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	ownerDone, webDone := make(chan error, 1), make(chan error, 1)
	go func() { ownerDone <- server.Serve(ctx, listener) }()
	<-listener.ready
	go func() { webDone <- webServer.Serve(udp) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-ownerDone:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(4 * time.Second):
			t.Error("owner/QUIC session did not stop")
		}
		_ = webServer.Close()
		_ = udp.Close()
		select {
		case <-webDone:
		case <-time.After(time.Second):
			t.Error("QUIC listener did not stop")
		}
	})
	return nativeWebTransportFixture{server: server, address: "https://" + udp.LocalAddr().String() + "/netplay", trust: tlsClient, pin: pin, ctx: ctx, match: match}
}

func TestWebTransportNativeDiscoveryFlushAndTLSAuthentication(t *testing.T) {
	match, _ := newDiscoveryTestMatch(t)
	fixture := startNativeWebTransportFixture(t, match, WebTransportOptions{})
	if transport, err := OpenWebTransport(fixture.ctx, fixture.address); err == nil {
		transport.Close()
		t.Fatal("untrusted self-signed certificate accepted")
	}
	expected, _ := match.ServerInfo()
	for range 3 {
		transport, err := openNativeWebTransport(fixture.ctx, fixture.address, fixture.trust)
		if err != nil {
			t.Fatal(err)
		}
		info, err := QueryServer(fixture.ctx, transport)
		if err != nil || !reflect.DeepEqual(info, expected) {
			t.Fatalf("reliable query closed before reply: info=%+v error=%v", info, err)
		}
	}
	if match.PlayerCount() != 0 {
		t.Fatal("discovery occupied player slot")
	}
}

func TestWebTransportNativeClientInputsAndCompressedSnapshots(t *testing.T) {
	match, world := newDiscoveryTestMatch(t)
	match.snapshots = codecSnapshotWorld{world, noisySnapshot(1).State}
	fixture := startNativeWebTransportFixture(t, match, WebTransportOptions{})
	transport, err := openNativeWebTransport(fixture.ctx, fixture.address, fixture.trust)
	if err != nil {
		t.Fatal(err)
	}
	client, err := Connect(fixture.ctx, transport, Hello{Compatibility: match.config.Compatibility, Name: "native QUIC"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	snapshot := nextWebSocketTestSnapshot(t, client)
	if len(snapshot.State) != 128<<10 {
		t.Fatal("full snapshot truncated")
	}
	target := snapshot.Tick + 3
	if err := client.SendInputs(InputBatch{Epoch: client.Welcome().Epoch, SnapshotAck: snapshot.ID, Inputs: []Input{{Sequence: target, Tick: target}}}); err != nil {
		t.Fatal(err)
	}
	for snapshot.Tick < target {
		snapshot = nextWebSocketTestSnapshot(t, client)
	}
	if !snapshot.Finalized.HasSequence || snapshot.Finalized.Sequence != target {
		t.Fatalf("datagram input not consumed: %+v", snapshot.Finalized)
	}
	if snapshot.State[0] != byte(snapshot.Tick) {
		t.Fatal("reconstructed datagram state differs")
	}
}

func TestWebTransportNativeCompletionFlushesFinalSnapshot(t *testing.T) {
	match, _ := newTestMatch(t)
	world := &completingTestWorld{completeAt: 2}
	match.world, match.snapshots, match.config.SnapshotInterval = world, world, 5
	fixture := startNativeWebTransportFixture(t, match, WebTransportOptions{})
	transport, err := openNativeWebTransport(fixture.ctx, fixture.address, fixture.trust)
	if err != nil {
		t.Fatal(err)
	}
	client, err := Connect(fixture.ctx, transport, Hello{Compatibility: "test-content", Name: "finish"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-client.done:
	case <-time.After(3 * time.Second):
		t.Fatal("completion not delivered")
	}
	snapshot, ok, err := client.PollSnapshot()
	if !ok || err != nil || snapshot.Tick != 2 {
		t.Fatalf("terminal state missing: %+v %v %v", snapshot, ok, err)
	}
	if client.Err() == nil {
		t.Fatal("missing completion notice")
	}
}

func TestWebTransportOriginAndSecureURL(t *testing.T) {
	for _, address := range []string{"http://localhost/netplay", "ws://localhost", "https://name:password@localhost", "https://localhost/#fragment"} {
		if _, err := webTransportURL(address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
	if url, err := webTransportURL("wt://example.test"); err != nil || url != "https://example.test/netplay" {
		t.Fatalf("URL=%q error=%v", url, err)
	}
	for _, test := range []struct {
		origin  string
		allowed bool
	}{{"", true}, {"https://server.test", true}, {"http://server.test", false}, {"https://play.test", true}, {"https://elsewhere.test", false}, {"https://play.test/path", false}} {
		request, _ := http.NewRequest("CONNECT", "https://server.test/netplay", nil)
		request.Header.Set("Origin", test.origin)
		if got := allowedWebTransportOrigin(request, []string{"https://play.test"}); got != test.allowed {
			t.Fatalf("origin %q allowed=%v", test.origin, got)
		}
	}
	if _, err := openNativeWebTransport(context.Background(), "https://localhost/netplay", &tls.Config{InsecureSkipVerify: true}); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("insecure TLS accepted")
	}
}

func TestWebTransportNativeOriginUpgradeAndCancellation(t *testing.T) {
	match, _ := newDiscoveryTestMatch(t)
	fixture := startNativeWebTransportFixture(t, match, WebTransportOptions{OriginPatterns: []string{"https://play.test"}})
	for _, test := range []struct {
		origin  string
		allowed bool
	}{{"https://play.test", true}, {"https://forbidden.test", false}} {
		ctx, cancel := context.WithTimeout(fixture.ctx, time.Second)
		dialer := &wt.Transport{TLSClientConfig: fixture.trust, QUICConfig: webTransportQUICConfig()}
		response, session, err := dialer.Dial(ctx, fixture.address, http.Header{"Origin": []string{test.origin}})
		if test.allowed {
			if err != nil {
				t.Fatal(err)
			}
			_ = session.CloseWithError(0, "")
		} else if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
			t.Fatalf("origin rejection: status=%v error=%v", response, err)
		}
		cancel()
		_ = dialer.Close()
	}
	for _, sendHello := range []bool{false, true} {
		ctx, cancel := context.WithCancel(fixture.ctx)
		transport, err := openNativeWebTransport(ctx, fixture.address, fixture.trust)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if sendHello {
			client, err := Connect(ctx, transport, Hello{Compatibility: match.config.Compatibility, Name: "cancel"})
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			cancel()
			_ = client.Close()
		} else {
			cancel()
		}
		select {
		case <-transport.(*webTransportTransport).closed:
		case <-time.After(time.Second):
			t.Fatal("canceled QUIC session did not drain")
		}
	}
}
