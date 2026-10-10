package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestLobbyCommandValidationAndHelp(t *testing.T) {
	for _, args := range [][]string{{}, {"-catalog", "catalog.json"}, {"-public-url", "https://play.example"}, {"-catalog", "catalog.json", "-public-url", "https://play.example", "-tls-cert", "certificate.pem"}, {"-catalog", "catalog.json", "-public-url", "https://play.example", "extra"}} {
		if err := run(context.Background(), args, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted incomplete CLI configuration: %v", args)
		}
	}
	var help bytes.Buffer
	if err := run(context.Background(), []string{"-help"}, io.Discard, &help); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-public-url", "-worker", "-max-rooms", "-upload-dir", "-upload-quota", "-download-quota", "-startup-timeout", "-idle-timeout", "-trusted-proxies"} {
		if !strings.Contains(help.String(), flag) {
			t.Fatalf("missing documented flag %s", flag)
		}
	}
}
