// gdlobby hosts a room directory and supervises one gdserver process per room.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gddoom/internal/roomhost"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "gdlobby: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("gdlobby", flag.ContinueOnError)
	fs.SetOutput(errOut)
	listen := fs.String("listen", "127.0.0.1:6670", "HTTP(S) listen address")
	public := fs.String("public-url", "", "public HTTP(S) origin for lobby and room URLs")
	catalog := fs.String("catalog", "", "operator JSON content catalog")
	worker := fs.String("worker", "gdserver", "authoritative server executable")
	origins := fs.String("web-origins", "", "comma-separated allowed browser origins, in addition to public-url")
	var trustedProxies trustedProxyFlags
	fs.Var(&trustedProxies, "trusted-proxies", "trusted literal loopback proxy IPs that replace X-Forwarded-For (repeatable or comma-separated)")
	cert := fs.String("tls-cert", "", "optional PEM TLS certificate")
	key := fs.String("tls-key", "", "optional PEM TLS private key")
	maxRooms := fs.Int("max-rooms", 8, "maximum simultaneous rooms, 1..32")
	idle := fs.Duration("idle-timeout", 10*time.Minute, "stop rooms after this time with no players, reserved bodies, or spectators")
	startup := fs.Duration("startup-timeout", 20*time.Second, "maximum worker startup time")
	poll := fs.Duration("poll-interval", 2*time.Second, "worker health and occupancy polling interval")
	shutdown := fs.Duration("shutdown-timeout", 5*time.Second, "graceful worker termination timeout")
	uploadDir := fs.String("upload-dir", "", "private persistent uploaded WAD storage; empty disables uploads")
	uploadQuota := fs.Int64("upload-quota", 512<<20, "maximum uploaded WAD storage in bytes")
	downloadQuota := fs.Int64("download-quota", 512<<20, "maximum immutable approved WAD download cache in bytes")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 || *catalog == "" || *public == "" {
		return errors.New("-catalog and -public-url are required; positional arguments are unsupported")
	}
	if (*cert == "") != (*key == "") {
		return errors.New("-tls-cert and -tls-key must be provided together")
	}
	packs, redistribution, err := roomhost.LoadCatalogWithPolicy(*catalog)
	if err != nil {
		return err
	}
	config := roomhost.Config{WorkerPath: *worker, PublicURL: *public, Packs: packs, MaxRooms: *maxRooms, IdleTimeout: *idle, StartupTimeout: *startup, PollInterval: *poll, ShutdownTimeout: *shutdown, UploadDir: *uploadDir, UploadQuota: *uploadQuota, Log: errOut}
	config.Redistribution, config.DownloadQuota = redistribution, *downloadQuota
	config.TrustedProxies = append([]string(nil), trustedProxies...)
	for _, origin := range strings.Split(*origins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			config.WebOrigins = append(config.WebOrigins, origin)
		}
	}
	manager, err := roomhost.New(ctx, config)
	if err != nil {
		return err
	}
	defer manager.Close()
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	if *cert != "" {
		certificate, err := tls.LoadX509KeyPair(*cert, *key)
		if err != nil {
			return err
		}
		listener = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	}
	server := &http.Server{Handler: manager.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 2 * time.Minute, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintf(out, "gdlobby: %s on %s, capacity %d\n", *public, listener.Addr(), *maxRooms)
	select {
	case <-ctx.Done():
	case err = <-done:
	}
	_ = manager.Close()
	_ = server.Close()
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

type trustedProxyFlags []string

func (values *trustedProxyFlags) String() string { return strings.Join(*values, ",") }

func (values *trustedProxyFlags) Set(value string) error {
	var parsed []string
	for _, entry := range strings.Split(value, ",") {
		ip, err := netip.ParseAddr(strings.TrimSpace(entry))
		if err != nil || ip.Zone() != "" || !ip.IsLoopback() {
			return errors.New("trusted proxy must be a literal loopback IP address")
		}
		parsed = append(parsed, ip.Unmap().String())
	}
	if len(*values)+len(parsed) > 16 {
		return errors.New("at most 16 trusted proxy addresses are supported")
	}
	*values = append(*values, parsed...)
	return nil
}
