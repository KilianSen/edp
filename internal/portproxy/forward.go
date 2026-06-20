package portproxy

import (
	"context"
	"io"
	"log"
	"net"
	"time"
)

// RunForwarder is the body of a forwarder sidecar: it listens on :listen and
// pipes every connection to target ("host:port"). It blocks until the process
// is killed. This is what `edp forward <listen> <target>` invokes.
func RunForwarder(listen, target string) {
	ln, err := net.Listen("tcp", ":"+listen)
	if err != nil {
		log.Fatalf("forward: listen :%s: %v", listen, err)
	}
	log.Printf("forward: :%s -> %s", listen, target)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("forward: accept: %v", err)
		}
		go pipe(conn, target)
	}
}

func pipe(client net.Conn, target string) {
	defer client.Close()
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var d net.Dialer
	upstream, err := d.DialContext(dialCtx, "tcp", target)
	if err != nil {
		return // target not up — drop the connection
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}
