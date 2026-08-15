//go:build integration

package network

import (
	"os"
	"testing"
)

// TestOpenLoopback exercises AF_PACKET setup. Run as root with: go test -tags integration ./internal/network.
func TestOpenLoopback(t *testing.T) {
	if os.Geteuid() != 0 { t.Skip("CAP_NET_RAW is required") }
	s, err := openSocket("lo")
	if err == nil { t.Fatal("loopback must be rejected because it has no Ethernet MAC") }
}
