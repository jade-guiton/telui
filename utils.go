package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
)

type server interface {
	Serve(net.Listener) error
}

func serveLocalhost(s server, desc string, port int) error {
	addresses := []string{
		fmt.Sprintf("127.0.0.1:%d", port),
		fmt.Sprintf("[::1]:%d", port),
	}
	for _, address := range addresses {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		go func() {
			err := s.Serve(listener)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintf(os.Stderr, "Fatal error in %s server: %v\n", desc, err)
			}
		}()
	}
	fmt.Printf("Started %s endpoint on port %d\n", desc, port)
	return nil
}
