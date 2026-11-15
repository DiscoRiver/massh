package massh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// OpenSSH configures running the system's OpenSSH client (ssh) for every job, instead of
// connecting with golang.org/x/crypto/ssh.
//
// This honours everything in the user's ssh_config (Match, Include, ProxyCommand, multi-hop
// ProxyJump, ControlMaster), and supports authentication the Go library doesn't, such as
// Kerberos, security keys and certificates. Config.SSHConfig, Config.BastionSSHConfig and
// Config.Resolve are not used. Each job runs in its own ssh process.
//
// massh runs ssh directly (never through a local shell), passes the host after "--" so it can't
// be read as an option, never lets it prompt on the terminal, and by default turns off agent,
// X11 and port forwarding even if ssh_config enables them.
type OpenSSH struct {
	// Path is the ssh program. Empty means "ssh" found on $PATH.
	Path string

	// ConfigFile, if set, replaces the user's ssh_config (ssh -F). It also applies to jump
	// hosts, which, as with ssh -J, don't use User or IdentityFiles.
	ConfigFile string

	// User is the remote user (ssh -l). Empty leaves it to ssh_config.
	User string

	// IdentityFiles are extra private keys to offer (ssh -i).
	IdentityFiles []string

	// HostKeyChecking sets StrictHostKeyChecking: "yes", "accept-new" or "no". Empty leaves it
	// to ssh_config. Options given here take precedence over ssh_config.
	HostKeyChecking string

	// ConnectTimeout bounds connecting, if positive.
	ConnectTimeout time.Duration

	// AllowForwarding keeps the agent, X11 and port forwarding settings from ssh_config. By
	// default they are turned off, since every host in a run would get them.
	AllowForwarding bool

	// Options are extra -o options, such as "ServerAliveInterval=15". The ones massh sets
	// itself take precedence.
	Options []string

	// Env is the environment for ssh. Nil means this process's environment. Remove secrets
	// from it: ssh passes its environment to ProxyCommand, and SendEnv can send it to hosts.
	Env []string

	// Password and Passphrase answer ssh's password and key passphrase prompts, through
	// AskpassProgram. Without either, ssh runs in batch mode and fails rather than prompt.
	Password   string
	Passphrase string

	// AskpassProgram is the program ssh runs to ask for a password. It must call
	// HandleAskpass first thing in main; usually it is the calling program itself
	// (os.Executable()). It is required if Password or Passphrase is set.
	AskpassProgram string
}

// Errors about OpenSSH configuration.
var (
	ErrUnsafeHost      = errors.New("massh: host name could be read as an ssh option, or contains spaces or control characters")
	ErrResolveWithSSH  = errors.New("massh: Resolve can't be used with OpenSSH; ssh_config applies instead")
	ErrNoAskpass       = errors.New("massh: OpenSSH.Password and Passphrase need OpenSSH.AskpassProgram")
	ErrBadHostKeyCheck = errors.New(`massh: OpenSSH.HostKeyChecking must be "", "yes", "accept-new" or "no"`)
)

// OpenSSHError is the error for a job whose ssh client failed (exit status 255): it couldn't
// connect, authenticate or verify the host key. A remote command that itself exits with
// status 255 is indistinguishable from this, which is a limitation of ssh.
type OpenSSHError struct {
	// Message is the last line ssh wrote to stderr, such as "Permission denied (publickey).".
	Message string
}

func (e *OpenSSHError) Error() string {
	if e.Message == "" {
		return "massh: ssh failed (exit status 255)"
	}
	return "massh: ssh: " + e.Message
}

// ExitError is the error for a job run through OpenSSH whose command exited with a non-zero
// status. See also [ExitStatus].
type ExitError struct {
	Status int
}

func (e *ExitError) Error() string { return fmt.Sprintf("massh: exit status %d", e.Status) }

func (o *OpenSSH) validate() []error {
	var errs []error
	switch o.HostKeyChecking {
	case "", "yes", "accept-new", "no":
	default:
		errs = append(errs, ErrBadHostKeyCheck)
	}
	if (o.Password != "" || o.Passphrase != "") && o.AskpassProgram == "" {
		errs = append(errs, ErrNoAskpass)
	}
	return errs
}

// safeForSSH reports whether host can be passed to ssh as a destination or -J value: it must not
// start with "-" (it could be read as an option, as in CVE-2017-1000117), and must not contain
// whitespace or control characters.
func safeForSSH(host string) bool {
	if host == "" || strings.HasPrefix(host, "-") {
		return false
	}
	return !strings.ContainsFunc(host, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

type opensshTransport struct {
	ctx     context.Context
	path    string
	o       *OpenSSH
	port    int // 0: leave it to ssh_config
	bastion string
	args    []string // options common to every invocation
	env     []string
	askpass *askpassServer
}

func newOpenSSHTransport(ctx context.Context, c *Config) (*opensshTransport, error) {
	o := c.OpenSSH
	path := o.Path
	if path == "" {
		path = "ssh"
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("massh: ssh client: %w", err)
	}
	t := &opensshTransport{ctx: ctx, path: resolved, o: o, port: c.Port, bastion: c.BastionHost}

	// ssh uses the first value it finds for each option, and command-line options come before
	// ssh_config, so these win over both.
	opt := func(kv string) { t.args = append(t.args, "-o", kv) }
	t.args = append(t.args, "-T") // never allocate a terminal
	if o.ConfigFile != "" {
		f, err := expandHome(o.ConfigFile)
		if err != nil {
			return nil, err
		}
		t.args = append(t.args, "-F", f)
	}
	opt("LogLevel=ERROR")
	if o.Password == "" && o.Passphrase == "" {
		opt("BatchMode=yes")
	} else {
		opt("BatchMode=no")
		opt("NumberOfPasswordPrompts=1")
	}
	if !o.AllowForwarding {
		opt("ForwardAgent=no")
		opt("ForwardX11=no")
		opt("ClearAllForwardings=yes")
		opt("PermitLocalCommand=no")
	}
	if o.HostKeyChecking != "" {
		opt("StrictHostKeyChecking=" + o.HostKeyChecking)
	}
	if o.ConnectTimeout > 0 {
		opt("ConnectTimeout=" + strconv.Itoa(max(int(o.ConnectTimeout.Round(time.Second)/time.Second), 1)))
	}
	for _, kv := range o.Options {
		opt(kv)
	}
	if o.User != "" {
		t.args = append(t.args, "-l", o.User)
	}
	for _, f := range o.IdentityFiles {
		f, err := expandHome(f)
		if err != nil {
			return nil, err
		}
		t.args = append(t.args, "-i", f)
	}
	if t.bastion != "" {
		t.args = append(t.args, "-J", t.bastion)
	}

	t.env = o.Env
	if t.env == nil {
		t.env = os.Environ()
	}
	if o.Password != "" || o.Passphrase != "" {
		t.askpass, err = newAskpassServer(o.Password, o.Passphrase)
		if err != nil {
			return nil, err
		}
		t.env = t.askpass.env(t.env, o.AskpassProgram)
	}
	return t, nil
}

func (t *opensshTransport) close() {
	if t.askpass != nil {
		t.askpass.close()
	}
}

func (t *opensshTransport) connect(host string) (hostConn, error) {
	return &opensshConn{t: t, host: host}, nil
}

type opensshConn struct {
	t    *opensshTransport
	host string
}

func (c *opensshConn) close() {}

// argv returns ssh's arguments for running command on the connection's host.
func (c *opensshConn) argv(command string) []string {
	args := append([]string(nil), c.t.args...)
	dest := c.host
	if h, p, err := net.SplitHostPort(c.host); err == nil {
		dest = h
		args = append(args, "-p", p)
	} else if c.t.port > 0 {
		args = append(args, "-p", strconv.Itoa(c.t.port))
	}
	dest = strings.TrimSuffix(strings.TrimPrefix(dest, "["), "]")
	// "--" ends the options, so the host can never be read as one.
	return append(args, "--", dest, command)
}

// start runs job in a new ssh process.
func (c *opensshConn) start(job Job) (*process, error) {
	cmd := exec.CommandContext(c.t.ctx, c.t.path, c.argv(job.Command)...)
	cmd.Env = c.t.env
	if job.Stdin != nil {
		cmd.Stdin = bytes.NewReader(job.Stdin)
	}
	// Kill ssh and anything it started, such as a ProxyCommand, when cancelled.
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
	cmd.WaitDelay = 2 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("massh: stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("massh: stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("massh: start ssh: %w", err)
	}
	tail := &tailBuffer{max: 4096}
	return &process{
		stdout: stdout,
		stderr: teeReader{stderrPipe, tail},
		wait: func() error {
			err := cmd.Wait()
			exitErr, ok := errors.AsType[*exec.ExitError](err)
			if !ok {
				return err
			}
			switch code := exitErr.ExitCode(); code {
			case 255:
				return &OpenSSHError{Message: tail.lastLine()}
			case -1:
				return fmt.Errorf("massh: ssh: %w", err) // killed by a signal
			default:
				return &ExitError{Status: code}
			}
		},
		kill: func() { _ = killProcessGroup(cmd) },
	}, nil
}

// teeReader copies what is read from r to w. Unlike io.TeeReader, w can't fail.
type teeReader struct {
	r io.Reader
	w *tailBuffer
}

func (t teeReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	t.w.Write(p[:n])
	return n, err
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
}

// lastLine returns the last non-empty line written.
func (b *tailBuffer) lastLine() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(b.buf)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
