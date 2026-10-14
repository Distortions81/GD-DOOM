package main

import "testing"

func TestUDPProxyRejectsPublicEndpointsAndOverlappingRoutes(t *testing.T) {
	var routes webProxyFlags
	native := udpNativeProxyFlags{routes: &routes}
	if err := native.Set("/deathmatch=tcp://127.0.0.1:6673"); err != nil {
		t.Fatal(err)
	}
	prefix := webProxyPrefixFlags{routes: &routes}
	if err := prefix.Set("/rooms/=http://127.0.0.1:6675/rooms/"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"/netplay=tcp://127.0.0.1:6673", "/deathmatch=tcp://127.0.0.1:6673", "/rooms/test=tcp://127.0.0.1:6673",
		"/public=tcp://203.0.113.1:6673", "/dns=tcp://localhost:6673", "/port=tcp://127.0.0.1:0", "/port=tcp://127.0.0.1:65536",
		"/credentials=tcp://user@127.0.0.1:6673", "/path=tcp://127.0.0.1:6673/game", "/query=tcp://127.0.0.1:6673?x=1",
		"/fragment=tcp://127.0.0.1:6673#x", "/wrong=http://127.0.0.1:6673", "/../dirty=tcp://127.0.0.1:6673",
	} {
		if err := native.Set(value); err == nil {
			t.Fatalf("accepted unsafe UDP route %q", value)
		}
	}
}
