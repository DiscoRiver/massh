package massh_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/discoriver/massh/v2"
	"github.com/discoriver/massh/v2/sshtest"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func newConfig(t *testing.T, srv *sshtest.Server, jobs ...massh.Job) *massh.Config {
	t.Helper()
	return &massh.Config{
		Hosts:     []string{srv.Addr},
		Jobs:      jobs,
		SSHConfig: srv.ClientConfig(),
	}
}

func cmd(c string) massh.Job { return massh.Job{Command: c} }

func TestRun(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd(`echo "Hello, World"; echo oops >&2; printf 'no newline'`))

	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	r := res[0]
	if r.Err != nil {
		t.Fatalf("Err = %v", r.Err)
	}
	if got, want := string(r.Stdout), "Hello, World\nno newline"; got != want {
		t.Errorf("Stdout = %q, want %q", got, want)
	}
	if got, want := string(r.Stderr), "oops\n"; got != want {
		t.Errorf("Stderr = %q, want %q", got, want)
	}
	if r.Host != srv.Addr || r.Job != 0 || r.Slow {
		t.Errorf("unexpected result metadata: %+v", r)
	}
}

func TestRunExitStatus(t *testing.T) {
	srv := sshtest.NewServer(t)
	res, err := newConfig(t, srv, cmd("echo partial; exit 3")).Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var exitErr *ssh.ExitError
	if !errors.As(res[0].Err, &exitErr) || exitErr.ExitStatus() != 3 {
		t.Fatalf("Err = %v, want exit status 3", res[0].Err)
	}
	if string(res[0].Stdout) != "partial\n" {
		t.Errorf("output of a failed command was lost: %q", res[0].Stdout)
	}
}

func TestRunManyHostsAndJobs(t *testing.T) {
	a, b := sshtest.NewServer(t), sshtest.NewServer(t)
	cfg := &massh.Config{
		// Duplicates are ignored.
		Hosts:     []string{a.Addr, b.Addr, a.Addr},
		Jobs:      []massh.Job{cmd("echo one"), cmd("echo two"), cmd("echo three")},
		SSHConfig: a.ClientConfig(),
		Workers:   1,
	}
	// Both servers must accept the same key and host key callback.
	cfg.SSHConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(a.ClientSigner, b.ClientSigner)}
	cfg.SSHConfig.HostKeyCallback = anyOf(a.HostKey, b.HostKey)

	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ host, out string }{
		{a.Addr, "one\n"}, {a.Addr, "two\n"}, {a.Addr, "three\n"},
		{b.Addr, "one\n"}, {b.Addr, "two\n"}, {b.Addr, "three\n"},
	}
	if len(res) != len(want) {
		t.Fatalf("got %d results, want %d", len(res), len(want))
	}
	for i, w := range want {
		if res[i].Err != nil || res[i].Host != w.host || string(res[i].Stdout) != w.out || res[i].Job != i%3 {
			t.Errorf("result %d = %+v, want host %s output %q", i, res[i], w.host, w.out)
		}
	}
	// Jobs on a host share one connection.
	if a.Conns() != 1 || b.Conns() != 1 {
		t.Errorf("connections: a=%d b=%d, want 1 each", a.Conns(), b.Conns())
	}
}

func TestRunUnreachableHost(t *testing.T) {
	srv := sshtest.NewServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().String()
	ln.Close()

	cfg := newConfig(t, srv, cmd("true"), cmd("true"))
	cfg.Hosts = append(cfg.Hosts, dead)
	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if (r.Host == dead) != (r.Err != nil) {
			t.Errorf("host %s job %d: Err = %v", r.Host, r.Job, r.Err)
		}
	}
}

func TestStreamEventOrder(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("for i in 1 2 3; do echo $i; echo e$i >&2; done"))

	events, err := cfg.Stream(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr []string
	var done int
	for ev := range events {
		if done > 0 {
			t.Fatalf("event %v after EventDone", ev.Kind)
		}
		switch ev.Kind {
		case massh.EventStdout:
			stdout = append(stdout, string(ev.Data))
		case massh.EventStderr:
			stderr = append(stderr, string(ev.Data))
		case massh.EventDone:
			done++
			if ev.Err != nil {
				t.Fatal(ev.Err)
			}
		default:
			t.Fatalf("unexpected event %v", ev.Kind)
		}
	}
	if got := strings.Join(stdout, ""); got != "1\n2\n3\n" {
		t.Errorf("stdout = %q", got)
	}
	if got := strings.Join(stderr, ""); got != "e1\ne2\ne3\n" {
		t.Errorf("stderr = %q", got)
	}
	if done != 1 {
		t.Errorf("got %d EventDone, want 1", done)
	}
}

func TestStreamLargeOutput(t *testing.T) {
	srv := sshtest.NewServer(t)
	// Write ~3.6MB to stderr before anything to stdout. That is more than the 2MB SSH channel
	// window, so it stalls if the two pipes aren't read concurrently.
	cfg := newConfig(t, srv, cmd("yes stderr-line | head -n 300000 >&2; echo end"))
	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Err != nil || string(res[0].Stdout) != "end\n" || strings.Count(string(res[0].Stderr), "\n") != 300000 {
		t.Fatalf("Err = %v, stdout = %q, %d stderr lines", res[0].Err, res[0].Stdout, strings.Count(string(res[0].Stderr), "\n"))
	}
}

func TestSlow(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("echo start; sleep 1; echo end"))
	cfg.SlowTimeout = 200 * time.Millisecond

	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Slow || res[0].Err != nil || string(res[0].Stdout) != "start\nend\n" {
		t.Fatalf("result = %+v, want slow and complete", res[0])
	}
}

func TestCancelSlow(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("echo start; sleep 30; echo end"), cmd("echo next"))
	cfg.SlowTimeout = 200 * time.Millisecond
	cfg.CancelSlow = true

	start := time.Now()
	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("slow job was not stopped (took %v)", time.Since(start))
	}
	if !res[0].Slow || !errors.Is(res[0].Err, massh.ErrSlow) || string(res[0].Stdout) != "start\n" {
		t.Errorf("slow result = %+v", res[0])
	}
	// Only the slow job is stopped.
	if res[1].Err != nil || string(res[1].Stdout) != "next\n" {
		t.Errorf("next result = %+v", res[1])
	}
}

func TestCancel(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("echo started; sleep 30"))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events, err := cfg.Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for ev := range events {
		if ev.Kind == massh.EventStdout {
			cancel()
		}
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("cancellation took %v", time.Since(start))
	}
}

func TestRunCancelled(t *testing.T) {
	srv := sshtest.NewServer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	res, err := newConfig(t, srv, cmd("sleep 30")).Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if !errors.Is(res[0].Err, context.DeadlineExceeded) {
		t.Errorf("result Err = %v, want deadline exceeded", res[0].Err)
	}
}

func TestBastion(t *testing.T) {
	bastion, target := sshtest.NewServer(t), sshtest.NewServer(t)
	cfg := newConfig(t, target, cmd("echo via bastion"))
	cfg.BastionHost = bastion.Addr
	cfg.BastionSSHConfig = bastion.ClientConfig()

	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Err != nil || string(res[0].Stdout) != "via bastion\n" {
		t.Fatalf("result = %+v", res[0])
	}
	if bastion.Forwards() != 1 {
		t.Errorf("bastion forwarded %d connections, want 1", bastion.Forwards())
	}
}

func TestBastionUnreachable(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("true"))
	cfg.BastionHost = "127.0.0.1:1"
	if _, err := cfg.Stream(t.Context()); err == nil {
		t.Fatal("Stream succeeded with an unreachable bastion")
	}
}

func TestScriptJob(t *testing.T) {
	srv := sshtest.NewServer(t)
	path := filepath.Join(t.TempDir(), "script.sh")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"[$a]\"; done\nexit 4\n"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := massh.NewScriptJob(path, "plain", "with space", "it's")
	if err != nil {
		t.Fatal(err)
	}

	res, err := newConfig(t, srv, job).Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(res[0].Stdout), "[plain]\n[with space]\n[it's]\n"; got != want {
		t.Errorf("Stdout = %q, want %q (stderr %q)", got, want, res[0].Stderr)
	}
	var exitErr *ssh.ExitError
	if !errors.As(res[0].Err, &exitErr) || exitErr.ExitStatus() != 4 {
		t.Errorf("Err = %v, want exit status 4", res[0].Err)
	}
	// The temporary script is removed.
	if left, _ := filepath.Glob(filepath.Join(srv.Dir, ".massh-script.*")); len(left) != 0 {
		t.Errorf("script files left behind: %v", left)
	}
}

func TestNewScriptJobErrors(t *testing.T) {
	dir := t.TempDir()
	noShebang := filepath.Join(dir, "a.sh")
	empty := filepath.Join(dir, "b.sh")
	os.WriteFile(noShebang, []byte("echo hi\n"), 0o600)
	os.WriteFile(empty, nil, 0o600)
	for _, p := range []string{noShebang, empty, filepath.Join(dir, "missing.sh")} {
		if _, err := massh.NewScriptJob(p); err == nil {
			t.Errorf("NewScriptJob(%s) succeeded", filepath.Base(p))
		}
	}
}

func TestValidate(t *testing.T) {
	err := (&massh.Config{}).Validate()
	for _, want := range []error{massh.ErrNoHosts, massh.ErrNoJobs, massh.ErrNoSSHConfig} {
		if !errors.Is(err, want) {
			t.Errorf("Validate() = %v, missing %v", err, want)
		}
	}
	err = (&massh.Config{Hosts: []string{"h"}, Jobs: []massh.Job{cmd("x")}, SSHConfig: &ssh.ClientConfig{}}).Validate()
	if !errors.Is(err, massh.ErrNoAuth) || !errors.Is(err, massh.ErrNoHostKeyCallback) {
		t.Errorf("Validate() = %v, want auth and host key errors", err)
	}
	if _, err := (&massh.Config{}).Run(t.Context()); err == nil {
		t.Error("Run succeeded with an invalid config")
	}
}

func TestPrivateKeyAuthAndKnownHosts(t *testing.T) {
	srv := sshtest.NewServer(t)
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(keyFile, srv.ClientKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	knownHostsFile := filepath.Join(dir, "known_hosts")
	line := knownHostsLine(srv.Addr, srv.HostKey)
	if err := os.WriteFile(knownHostsFile, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	auth, err := massh.PrivateKeyAuth(keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	hostKeys, err := massh.KnownHosts(knownHostsFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := newConfig(t, srv, cmd("echo ok"))
	cfg.SSHConfig = &ssh.ClientConfig{User: sshtest.User, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: hostKeys}
	res, err := cfg.Run(t.Context())
	if err != nil || res[0].Err != nil {
		t.Fatalf("Run: %v, %v", err, res[0].Err)
	}

	// An unknown host key is rejected.
	other := sshtest.NewServer(t)
	cfg.Hosts = []string{other.Addr}
	res, _ = cfg.Run(t.Context())
	if res[0].Err == nil {
		t.Error("connected to a host that isn't in known_hosts")
	}

	if _, err := massh.PrivateKeyAuth(filepath.Join(dir, "missing"), ""); err == nil {
		t.Error("PrivateKeyAuth succeeded with a missing file")
	}
	if _, err := massh.PrivateKeyAuth(keyFile, "not-encrypted"); err == nil {
		t.Error("PrivateKeyAuth succeeded with a passphrase for an unencrypted key")
	}
}

func anyOf(keys ...ssh.PublicKey) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		for _, k := range keys {
			if string(k.Marshal()) == string(key.Marshal()) {
				return nil
			}
		}
		return errors.New("unknown host key")
	}
}

func TestAgentAuth(t *testing.T) {
	srv := sshtest.NewServer(t)
	keyring := agent.NewKeyring()
	signerKey, err := ssh.ParseRawPrivateKey(srv.ClientKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: signerKey}); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, c)
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)

	auth, err := massh.AgentAuth()
	if err != nil {
		t.Fatal(err)
	}
	cfg := newConfig(t, srv, cmd("echo ok"))
	cfg.SSHConfig.Auth = []ssh.AuthMethod{auth}
	res, err := cfg.Run(t.Context())
	if err != nil || res[0].Err != nil {
		t.Fatalf("Run: %v, %v", err, res[0].Err)
	}

	t.Setenv("SSH_AUTH_SOCK", "")
	if _, err := massh.AgentAuth(); err == nil {
		t.Error("AgentAuth succeeded without SSH_AUTH_SOCK")
	}
}

func TestLongLinesAreSplit(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("head -c 200000 /dev/zero | tr '\\0' x; echo; echo short"))
	events, err := cfg.Stream(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	var chunks int
	for ev := range events {
		if ev.Kind == massh.EventStdout {
			if len(ev.Data) > massh.MaxLineLength {
				t.Errorf("event of %d bytes exceeds MaxLineLength", len(ev.Data))
			}
			out = append(out, ev.Data...)
			chunks++
		}
	}
	if want := strings.Repeat("x", 200000) + "\nshort\n"; string(out) != want {
		t.Errorf("reassembled output differs (%d bytes, want %d)", len(out), len(want))
	}
	if chunks < 4 {
		t.Errorf("got %d events, want the long line split", chunks)
	}
}

func TestValidateEmptyHost(t *testing.T) {
	srv := sshtest.NewServer(t)
	cfg := newConfig(t, srv, cmd("true"))
	cfg.Hosts = append(cfg.Hosts, " ")
	if err := cfg.Validate(); !errors.Is(err, massh.ErrEmptyHost) {
		t.Errorf("Validate() = %v, want ErrEmptyHost", err)
	}
}

func TestResolve(t *testing.T) {
	bastion, target := sshtest.NewServer(t), sshtest.NewServer(t)
	cfg := &massh.Config{
		Hosts:     []string{"alias-a", "alias-b", "broken"},
		Jobs:      []massh.Job{cmd("echo ok")},
		SSHConfig: bastion.ClientConfig(), // deliberately wrong for target
		Resolve: func(host string) (massh.HostOptions, error) {
			switch host {
			case "alias-a", "alias-b":
				return massh.HostOptions{
					Addr:             target.Addr,
					SSHConfig:        target.ClientConfig(),
					Bastion:          bastion.Addr,
					BastionSSHConfig: bastion.ClientConfig(),
				}, nil
			}
			return massh.HostOptions{}, errors.New("no such alias")
		},
	}
	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res[:2] {
		if r.Err != nil || string(r.Stdout) != "ok\n" {
			t.Errorf("%s: %+v", r.Host, r)
		}
	}
	if res[2].Err == nil || !strings.Contains(res[2].Err.Error(), "no such alias") {
		t.Errorf("broken: Err = %v", res[2].Err)
	}
	// The bastion connection is shared.
	if bastion.Conns() != 1 || bastion.Forwards() != 2 {
		t.Errorf("bastion conns = %d, forwards = %d; want 1 and 2", bastion.Conns(), bastion.Forwards())
	}
}

func TestKnownHostsAcceptNew(t *testing.T) {
	srv := sshtest.NewServer(t)
	file := filepath.Join(t.TempDir(), "sub", "known_hosts")
	cb, err := massh.KnownHostsAcceptNew(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg := newConfig(t, srv, cmd("true"))
	cfg.SSHConfig.HostKeyCallback = cb

	// First contact: accepted and recorded.
	if res, _ := cfg.Run(t.Context()); res[0].Err != nil {
		t.Fatalf("first connection: %v", res[0].Err)
	}
	data, _ := os.ReadFile(file)
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("known_hosts = %q, want one line", data)
	}
	// Second contact: known and matching.
	if res, _ := cfg.Run(t.Context()); res[0].Err != nil {
		t.Fatalf("second connection: %v", res[0].Err)
	}
	// A different key for the same address is rejected.
	key := sshtest.NewServer(t).HostKey
	remote, _ := net.ResolveTCPAddr("tcp", srv.Addr)
	if err := cb(srv.Addr, remote, key); err == nil {
		t.Error("accepted a changed host key")
	}
	data2, _ := os.ReadFile(file)
	if string(data2) != string(data) {
		t.Error("known_hosts changed after a key mismatch")
	}
}
