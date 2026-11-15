package massh

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// hostAddr returns host as "host:port", adding port if host doesn't have one.
func hostAddr(host string, port int) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"), strconv.Itoa(port))
}

// dial connects to addr, through bastion if it is non-nil, and performs the SSH handshake. The
// connection and handshake are bounded by ctx and by cfg.Timeout.
func dial(ctx context.Context, bastion *ssh.Client, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	var conn net.Conn
	var err error
	if bastion != nil {
		conn, err = bastion.DialContext(ctx, "tcp", addr)
	} else {
		var d net.Dialer
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("massh: dial %s: %w", addr, err)
	}

	// ssh.NewClientConn has no context, so interrupt it by closing the connection.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if !stop() {
		if err == nil {
			c.Close()
		}
		return nil, fmt.Errorf("massh: ssh handshake with %s: %w", addr, context.Cause(ctx))
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("massh: ssh handshake with %s: %w", addr, err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}
