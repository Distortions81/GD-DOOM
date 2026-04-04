package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gddoom/internal/netplay"
)

func main() {
	addr := flag.String("listen", ":6670", "TCP listen address")
	flag.Parse()

	srv, err := netplay.ListenServer(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gdsfrelay: %v\n", err)
		os.Exit(1)
	}
	defer srv.Close()

	fmt.Fprintf(os.Stderr, "gdsfrelay: listening on %s\n", srv.Addr())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	<-sigc
}
