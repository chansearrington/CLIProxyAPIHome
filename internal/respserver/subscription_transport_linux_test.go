//go:build linux

package respserver

import (
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPIHome/internal/protocolmux"
	"golang.org/x/sys/unix"
)

func TestSubscriptionTransportBoundsWrappedTCPSocket(t *testing.T) {
	listener, errListen := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if errListen != nil {
		t.Fatal(errListen)
	}
	defer func() {
		if errClose := listener.Close(); errClose != nil {
			t.Errorf("close listener: %v", errClose)
		}
	}()
	peer, errDial := net.Dial("tcp", listener.Addr().String())
	if errDial != nil {
		t.Fatal(errDial)
	}
	defer func() {
		if errClose := peer.Close(); errClose != nil {
			t.Errorf("close peer: %v", errClose)
		}
	}()
	conn, errAccept := listener.AcceptTCP()
	if errAccept != nil {
		t.Fatal(errAccept)
	}
	defer func() {
		if errClose := conn.Close(); errClose != nil && !errors.Is(errClose, net.ErrClosed) {
			t.Errorf("close transport: %v", errClose)
		}
	}()

	// Match the protocol mux's outer buffer, TLS, and inner buffer layers.
	wrapped := &protocolmux.BufferedConn{Conn: tls.Server(&protocolmux.BufferedConn{Conn: conn}, &tls.Config{})}
	raw, errRaw := conn.SyscallConn()
	if errRaw != nil {
		t.Fatal(errRaw)
	}
	for _, test := range []struct {
		timeout time.Duration
		want    int
	}{
		{3 * time.Second, 3000},
		{time.Nanosecond, 1},
		{1000 * time.Hour, 1<<31 - 1},
	} {
		if errBound := boundSubscriptionTransport(wrapped, test.timeout); errBound != nil {
			t.Fatal(errBound)
		}
		var milliseconds int
		var errOption error
		if errControl := raw.Control(func(fd uintptr) {
			milliseconds, errOption = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT)
		}); errControl != nil {
			t.Fatal(errControl)
		}
		if errOption != nil {
			t.Fatal(errOption)
		}
		if milliseconds != test.want {
			t.Fatalf("TCP_USER_TIMEOUT for %v = %d ms, want %d", test.timeout, milliseconds, test.want)
		}
	}
	if errClose := conn.Close(); errClose != nil {
		t.Fatal(errClose)
	}
	if errBound := boundSubscriptionTransport(wrapped, 3*time.Second); errBound == nil {
		t.Fatal("closed TCP socket accepted timeout configuration")
	}
}

func TestSubscriptionTransportAllowsNonTCPTestConnections(t *testing.T) {
	left, right := net.Pipe()
	defer func() {
		if errClose := left.Close(); errClose != nil {
			t.Errorf("close left: %v", errClose)
		}
		if errClose := right.Close(); errClose != nil {
			t.Errorf("close right: %v", errClose)
		}
	}()
	if errBound := boundSubscriptionTransport(left, 3*time.Second); errBound != nil {
		t.Fatal(errBound)
	}
}
