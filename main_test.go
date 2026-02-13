package main

import (
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/cjeanner/kustomap/internal/server"
	"github.com/cjeanner/kustomap/internal/storage"
)

func TestParsePort(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{"default", "3000", 3000, false},
		{"min valid", "1", 1, false},
		{"max valid", "65535", 65535, false},
		{"empty", "", 0, true},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"over max", "65536", 0, true},
		{"not a number", "abc", 0, true},
		{"with spaces", " 3000 ", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parsePort(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parsePort(%q) expected error, got %d", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePort(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("parsePort(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestServerListensOnPort(t *testing.T) {
	// Use a dynamic port to avoid conflicts.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	portStr := strconv.Itoa(port)
	portNum, err := parsePort(portStr)
	if err != nil {
		t.Fatalf("parsePort(%q): %v", portStr, err)
	}
	if portNum != port {
		t.Fatalf("parsePort(%q) = %d, want %d", portStr, portNum, port)
	}

	store := storage.NewMemoryStorage()
	webRoot, _ := fs.Sub(webFS, "web")
	r := server.New(store, webRoot, nil)
	addr := ":" + strconv.Itoa(portNum)
	listener, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	defer listener.Close()

	go func() {
		_ = http.Serve(listener, r)
	}()

	// Use a valid UUID that is not in the store → 404
	nonexistentUUID := "00000000-0000-4000-8000-000000000001"
	resp, err := http.Get("http://127.0.0.1:" + portStr + "/api/v1/graph/" + nonexistentUUID)
	if err != nil {
		t.Fatalf("GET (server should be listening): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/v1/graph/%s status = %d, want 404", nonexistentUUID, resp.StatusCode)
	}
}
