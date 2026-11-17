package massh

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"sync"

	"golang.org/x/crypto/ssh"
)

// nativeTransport connects with golang.org/x/crypto/ssh: one connection per host, with jobs in
// sessions on it.
type nativeTransport struct {
	ctx              context.Context
	sshConfig        *ssh.ClientConfig
	bastionHost      string
	bastionSSHConfig *ssh.ClientConfig
	resolve          func(string) (HostOptions, error)
	port             int

	mu       sync.Mutex
	bastions map[bastionKey]*bastionConn
}

type bastionKey struct{ addr, user string }

// bastionConn is a connection to a bastion host, shared by every host that uses it.
type bastionConn struct {
	once   sync.Once
	client *ssh.Client
	err    error
}

func newNativeTransport(ctx context.Context, c *Config) (*nativeTransport, error) {
	t := &nativeTransport{
		ctx:              ctx,
		sshConfig:        c.SSHConfig,
		bastionHost:      c.BastionHost,
		bastionSSHConfig: c.bastionSSHConfig(),
		resolve:          c.Resolve,
		port:             c.port(),
		bastions:         make(map[bastionKey]*bastionConn),
	}
	// Connect to the default bastion now, so that a problem with it is reported by Stream.
	if c.BastionHost != "" {
		if _, err := t.bastion(c.BastionHost, t.bastionSSHConfig); err != nil {
			t.close()
			return nil, fmt.Errorf("massh: bastion: %w", err)
		}
	}
	return t, nil
}

func (t *nativeTransport) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, b := range t.bastions {
		if b.client != nil {
			b.client.Close()
		}
	}
}

// bastion returns the shared connection to the bastion host, connecting on first use.
func (t *nativeTransport) bastion(host string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	addr := hostAddr(host, t.port)
	key := bastionKey{addr, cfg.User}
	t.mu.Lock()
	b, ok := t.bastions[key]
	if !ok {
		b = &bastionConn{}
		t.bastions[key] = b
	}
	t.mu.Unlock()
	b.once.Do(func() { b.client, b.err = dial(t.ctx, nil, addr, cfg) })
	return b.client, b.err
}

// connect resolves host's options and connects to it.
func (t *nativeTransport) connect(host string) (hostConn, error) {
	var opts HostOptions
	if t.resolve != nil {
		var err error
		if opts, err = t.resolve(host); err != nil {
			return nil, fmt.Errorf("massh: resolve %s: %w", host, err)
		}
	}
	cfg := cmp.Or(opts.SSHConfig, t.sshConfig)
	if err := validateSSHConfig(cfg); err != nil {
		return nil, err
	}

	bastionHost, bastionCfg := t.bastionHost, t.bastionSSHConfig
	if opts.Bastion != "" && opts.Bastion != t.bastionHost {
		bastionHost, bastionCfg = opts.Bastion, cfg
	}
	if opts.BastionSSHConfig != nil {
		bastionCfg = opts.BastionSSHConfig
	}
	var bastion *ssh.Client
	if bastionHost != "" {
		if err := validateSSHConfig(bastionCfg); err != nil {
			return nil, fmt.Errorf("massh: bastion %s: %w", bastionHost, err)
		}
		var err error
		if bastion, err = t.bastion(bastionHost, bastionCfg); err != nil {
			return nil, fmt.Errorf("massh: bastion %s: %w", bastionHost, err)
		}
	}
	client, err := dial(t.ctx, bastion, hostAddr(cmp.Or(opts.Addr, host), t.port), cfg)
	if err != nil {
		return nil, err
	}
	// Closing the connection is the only reliable way to interrupt a running command.
	stop := context.AfterFunc(t.ctx, func() { client.Close() })
	return &nativeConn{client: client, stop: stop}, nil
}

type nativeConn struct {
	client *ssh.Client
	stop   func() bool
}

func (c *nativeConn) close() {
	c.stop()
	c.client.Close()
}

// start runs job in a new session.
func (c *nativeConn) start(job Job) (*process, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("massh: new session: %w", err)
	}
	if job.Stdin != nil {
		sess.Stdin = bytes.NewReader(job.Stdin)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("massh: stdout: %w", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("massh: stderr: %w", err)
	}
	if err := sess.Start(job.Command); err != nil {
		sess.Close()
		return nil, fmt.Errorf("massh: start: %w", err)
	}
	return &process{
		stdout: stdout,
		stderr: stderr,
		wait: func() error {
			defer sess.Close()
			return sess.Wait()
		},
		kill: func() {
			_ = sess.Signal(ssh.SIGKILL)
			_ = sess.Close()
		},
	}, nil
}
