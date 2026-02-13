package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/cjeanner/kustomap/internal/cacert"
	"github.com/cjeanner/kustomap/internal/server"
	"github.com/cjeanner/kustomap/internal/storage"
)

const defaultPort = 3000

// Valid port range for TCP (IANA: 1-65535).
const minPort = 1
const maxPort = 65535

//go:embed web
var webFS embed.FS

func main() {
	portFlag := flag.String("port", "", "HTTP listener port (default 3000, or set PORT env)")
	flag.Parse()

	portStr := *portFlag
	if portStr == "" {
		portStr = os.Getenv("PORT")
	}
	if portStr == "" {
		portStr = strconv.Itoa(defaultPort)
	}
	port, err := parsePort(portStr)
	if err != nil {
		log.Fatalf("invalid port: %v", err)
	}

	store := storage.NewMemoryStorage()
	caCollector := cacert.NewCollector(cacert.DefaultTTL)
	webRoot, _ := fs.Sub(webFS, "web")
	r := server.New(store, webRoot, caCollector)

	addr := ":" + strconv.Itoa(port)
	log.Printf("🚀 Server listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// parsePort validates and returns the port number. Accepts a positive integer
// in the standard TCP port range 1-65535.
func parsePort(s string) (int, error) {
	if s == "" {
		return 0, errors.New("port is required")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("port must be an integer: %w", err)
	}
	if n < minPort || n > maxPort {
		return 0, fmt.Errorf("port must be between %d and %d", minPort, maxPort)
	}
	return n, nil
}
