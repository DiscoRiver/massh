// Package sshtest provides an in-process SSH server for testing code that uses massh, much as
// net/http/httptest does for HTTP.
//
// The server runs each "exec" request with "sh -c" on the local machine, and supports
// "direct-tcpip" forwarding so that it can act as a bastion host. It only accepts the key in
// [Server.ClientSigner] and listens on the loopback interface.
package sshtest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// User is the user name that clients of a Server should use. Any name is accepted.
const User = "massh-test"

// Server is an SSH server listening on a loopback address.
type Server struct {
	// Addr is the server's address, in the form "127.0.0.1:port".
	Addr string
	// HostKey is the server's public host key.
	HostKey ssh.PublicKey
	// ClientSigner is the only client key the server accepts.
	ClientSigner ssh.Signer
	// ClientKeyPEM is ClientSigner's private key in OpenSSH PEM format, for tests that need a
	// key file.
	ClientKeyPEM []byte
	// Dir is the working directory of commands run by the server.
	Dir string

	listener net.Listener
	config   *ssh.ServerConfig
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	conns    atomic.Int64
	forwards atomic.Int64
}

// NewServer starts a Server, which is closed when the test finishes.
func NewServer(tb testing.TB) *Server {
	tb.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		tb.Fatal(err)
	}
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		tb.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		tb.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		Addr:         ln.Addr().String(),
		HostKey:      hostSigner.PublicKey(),
		ClientSigner: clientSigner,
		ClientKeyPEM: pem.EncodeToMemory(block),
		Dir:          tb.TempDir(),
		listener:     ln,
		ctx:          ctx,
		cancel:       cancel,
	}

	clientKey := clientSigner.PublicKey().Marshal()
	s.config = &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(clientKey) {
				return nil, nil
			}
			return nil, errors.New("sshtest: unknown key")
		},
	}
	s.config.AddHostKey(hostSigner)

	s.wg.Go(s.serve)
	tb.Cleanup(s.Close)
	return s
}

// ClientConfig returns a client config that authenticates with ClientSigner and accepts only the
// server's host key.
func (s *Server) ClientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.ClientSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.HostKey),
		Timeout:         5 * time.Second,
	}
}

// Conns returns the number of SSH connections the server has accepted.
func (s *Server) Conns() int { return int(s.conns.Load()) }

// Forwards returns the number of "direct-tcpip" channels the server has opened.
func (s *Server) Forwards() int { return int(s.forwards.Load()) }

// Close stops the server, closes all its connections and waits for their commands to exit.
func (s *Server) Close() {
	s.cancel()
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) serve() {
	for {
		nc, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Go(func() { s.handleConn(nc) })
	}
}

func (s *Server) handleConn(nc net.Conn) {
	sc, chans, reqs, err := ssh.NewServerConn(nc, s.config)
	if err != nil {
		nc.Close()
		return
	}
	s.conns.Add(1)

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { sc.Close() })
	defer stop()
	go ssh.DiscardRequests(reqs)

	var wg sync.WaitGroup
	for nch := range chans {
		switch nch.ChannelType() {
		case "session":
			wg.Go(func() { s.handleSession(ctx, nch) })
		case "direct-tcpip":
			wg.Go(func() { s.handleForward(ctx, nch) })
		default:
			nch.Reject(ssh.UnknownChannelType, "unsupported channel type")
		}
	}
	// The client has gone: kill anything still running.
	cancel()
	wg.Wait()
}

func (s *Server) handleSession(ctx context.Context, nch ssh.NewChannel) {
	ch, reqs, err := nch.Accept()
	if err != nil {
		return
	}
	defer ch.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var running sync.WaitGroup
	defer running.Wait()
	started := false
	for req := range reqs {
		switch req.Type {
		case "exec":
			var p struct{ Command string }
			if started || ssh.Unmarshal(req.Payload, &p) != nil {
				req.Reply(false, nil)
				continue
			}
			started = true
			req.Reply(true, nil)
			running.Go(func() {
				status := s.exec(ctx, ch, p.Command)
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				ch.Close()
			})
		case "signal":
			cancel()
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
	// The client closed the channel.
	cancel()
}

func (s *Server) exec(ctx context.Context, ch ssh.Channel, command string) uint32 {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = s.Dir
	cmd.Stdin = ch
	cmd.Stdout = ch
	cmd.Stderr = ch.Stderr()
	// Don't wait forever for stdin, or for pipes held open by orphaned children.
	cmd.WaitDelay = time.Second

	err := cmd.Run()
	if cmd.ProcessState == nil {
		return 127
	}
	if code := cmd.ProcessState.ExitCode(); code >= 0 {
		return uint32(code)
	}
	if err != nil {
		return 255
	}
	return 0
}

func (s *Server) handleForward(ctx context.Context, nch ssh.NewChannel) {
	var p struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(nch.ExtraData(), &p); err != nil {
		nch.Reject(ssh.ConnectionFailed, "bad request")
		return
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(p.Host, strconv.Itoa(int(p.Port))))
	if err != nil {
		nch.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := nch.Accept()
	if err != nil {
		conn.Close()
		return
	}
	s.forwards.Add(1)
	go ssh.DiscardRequests(reqs)

	stop := context.AfterFunc(ctx, func() {
		conn.Close()
		ch.Close()
	})
	defer stop()

	var wg sync.WaitGroup
	wg.Go(func() {
		io.Copy(conn, ch)
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	})
	wg.Go(func() {
		io.Copy(ch, conn)
		ch.CloseWrite()
	})
	wg.Wait()
	conn.Close()
	ch.Close()
}
