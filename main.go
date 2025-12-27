package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"school-with-ai/components"

	"github.com/a-h/templ"
)

func main() {
	component := components.Hello("John")

	http.Handle("/", templ.Handler(component))

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		panic("SERVER_ADDR environment variable is not set")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(fmt.Sprintf("invalid SERVER_ADDR: %v", err))
	}

	slog.Info("Starting server", "host", host, "port", port)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}
