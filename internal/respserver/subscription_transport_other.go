//go:build !linux

package respserver

import (
	"net"
	"time"
)

// Linux supplies TCP_USER_TIMEOUT; other platforms retain their TCP defaults.
func boundSubscriptionTransport(net.Conn, time.Duration) error {
	return nil
}
