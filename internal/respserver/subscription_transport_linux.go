//go:build linux

package respserver

import (
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// boundSubscriptionTransport stops a dead peer from retaining its membership
// while heartbeat writes continue to succeed into the local TCP send buffer.
// This bounds unacknowledged TCP data, not idle subscriptions or inference.
func boundSubscriptionTransport(conn net.Conn, timeout time.Duration) error {
	milliseconds := timeout / time.Millisecond
	if timeout%time.Millisecond != 0 {
		milliseconds++
	}
	if milliseconds < 1 {
		milliseconds = 1
	} else if milliseconds > 1<<31-1 {
		milliseconds = 1<<31 - 1
	}
	for depth := 0; conn != nil && depth < 8; depth++ {
		if tcp, ok := conn.(*net.TCPConn); ok {
			raw, errRaw := tcp.SyscallConn()
			if errRaw != nil {
				return errRaw
			}
			var errOption error
			errControl := raw.Control(func(fd uintptr) {
				errOption = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(milliseconds))
			})
			if errControl != nil {
				return errControl
			}
			return errOption
		}
		wrapped, ok := conn.(interface{ NetConn() net.Conn })
		if !ok {
			return nil
		}
		conn = wrapped.NetConn()
	}
	return nil
}
