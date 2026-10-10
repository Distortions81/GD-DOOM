package main

import (
	"context"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestTrustedProxyFlagRepeatedAndCommaSeparated(t *testing.T) {
	var proxies trustedProxyFlags
	fs := flag.NewFlagSet("proxy-test", flag.ContinueOnError)
	fs.Var(&proxies, "trusted-proxies", "")
	if err := fs.Parse([]string{"-trusted-proxies", "127.0.0.1, ::1", "-trusted-proxies", "::ffff:127.0.0.2"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string(proxies), []string{"127.0.0.1", "::1", "127.0.0.2"}) {
		t.Fatalf("trusted proxy flags not combined: %v", proxies)
	}
	before := proxies.String()
	if err := proxies.Set("127.0.0.1,localhost"); err == nil || proxies.String() != before {
		t.Fatal("invalid flag partially changed trusted proxies")
	}
}

func TestTrustedProxyFlagRejectsUnsafeConfigurationBeforeLoadingCatalog(t *testing.T) {
	for _, value := range []string{"", "*", "localhost", "127.0.0.1:443", "127.0.0.0/8", "192.0.2.1", "::1%lo", "127.0.0.1,"} {
		err := run(context.Background(), []string{"-catalog", "missing.json", "-public-url", "https://play.example", "-trusted-proxies", value}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "trusted proxy") {
			t.Fatalf("unsafe flag %q accepted: %v", value, err)
		}
	}
}
