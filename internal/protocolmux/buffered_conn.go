package protocolmux

import (
	"bufio"
	"crypto/tls"
	"net"
)

type BufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// NetConn returns the transport without discarding buffered reads.
func (c *BufferedConn) NetConn() net.Conn {
	if c == nil {
		return nil
	}
	return c.Conn
}

// Read loads read.
func (c *BufferedConn) Read(p []byte) (int, error) {
	if c == nil {
		return 0, net.ErrClosed
	}
	if c.reader == nil {
		return c.Conn.Read(p)
	}
	return c.reader.Read(p)
}

// ConnectionState handles a connection state.
func (c *BufferedConn) ConnectionState() tls.ConnectionState {
	if c == nil || c.Conn == nil {
		return tls.ConnectionState{}
	}
	if stater, ok := c.Conn.(interface{ ConnectionState() tls.ConnectionState }); ok {
		return stater.ConnectionState()
	}
	return tls.ConnectionState{}
}
