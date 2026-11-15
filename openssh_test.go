package massh_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/discoriver/massh/v2"
	"github.com/discoriver/massh/v2/sshtest"
	"golang.org/x/crypto/ssh/knownhosts"
)

// TestMain lets the test binary act as the askpass helper for the OpenSSH tests.
func TestMain(m *testing.M) {
	massh.HandleAskpass()
	os.Exit(m.Run())
}

// fakeSSH is a stand-in for ssh: it records its arguments, fails like ssh for the hosts
// "unreachable" (can't connect) and "locked" (needs the password in $FAKE_SSH_PASSWORD, asked
// for through $SSH_ASKPASS), and otherwise runs the command locally.
const fakeSSH = `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_SSH_LOG.$$"
while [ $# -gt 0 ] && [ "$1" != "--" ]; do shift; done
shift
host=$1
cmd=$2
case "$host" in
unreachable)
	echo "ssh: connect to host unreachable port 22: Connection refused" >&2
	exit 255 ;;
locked)
	pw=$("$SSH_ASKPASS" "massh-test@locked's password: ") || { echo "Permission denied (password)." >&2; exit 255; }
	[ "$pw" = "$FAKE_SSH_PASSWORD" ] || { echo "Permission denied (password)." >&2; exit 255; }
	;;
esac
exec sh -c "$cmd"
`

type fake struct {
	path, logPrefix string
	env             []string
}

func newFakeSSH(t *testing.T) fake {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh")
	if err := os.WriteFile(path, []byte(fakeSSH), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "args")
	return fake{path: path, logPrefix: log, env: append(os.Environ(), "FAKE_SSH_LOG="+log, "FAKE_SSH_PASSWORD=hunter2")}
}

// invocations returns the argument lists ssh was run with.
func (f fake) invocations(t *testing.T) [][]string {
	t.Helper()
	files, _ := filepath.Glob(f.logPrefix + ".*")
	var out [][]string
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"))
	}
	return out
}

func (f fake) config(hosts []string, jobs ...massh.Job) *massh.Config {
	return &massh.Config{
		Hosts:   hosts,
		Jobs:    jobs,
		OpenSSH: &massh.OpenSSH{Path: f.path, Env: f.env, HostKeyChecking: "yes"},
	}
}

func TestOpenSSHFake(t *testing.T) {
	f := newFakeSSH(t)
	cfg := f.config([]string{"web1", "web2:2222", "unreachable"},
		cmd("echo out; echo err >&2"), cmd("exit 3"))
	cfg.BastionHost = "jump.example.com"

	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if r := res[0]; r.Err != nil || string(r.Stdout) != "out\n" || string(r.Stderr) != "err\n" {
		t.Errorf("web1 job 0 = %+v", r)
	}
	if status, ok := massh.ExitStatus(res[1].Err); !ok || status != 3 {
		t.Errorf("web1 job 1: Err = %v, want exit status 3", res[1].Err)
	}
	for _, r := range res[4:] {
		sshErr, ok := errors.AsType[*massh.OpenSSHError](r.Err)
		if !ok || !strings.Contains(sshErr.Message, "Connection refused") {
			t.Errorf("unreachable: Err = %v, want an OpenSSHError about the connection", r.Err)
		}
		if _, ok := massh.ExitStatus(r.Err); ok {
			t.Error("a connection failure has an exit status")
		}
	}

	runs := f.invocations(t)
	if len(runs) != 6 {
		t.Fatalf("ssh ran %d times, want one per host and job (6)", len(runs))
	}
	for _, args := range runs {
		i := slices.Index(args, "--")
		if i < 0 || i != len(args)-3 {
			t.Fatalf("host and command don't follow --: %q", args)
		}
		opts := strings.Join(args[:i], " ")
		for _, want := range []string{"-T", "BatchMode=yes", "ForwardAgent=no", "ForwardX11=no",
			"ClearAllForwardings=yes", "PermitLocalCommand=no", "StrictHostKeyChecking=yes", "-J jump.example.com"} {
			if !strings.Contains(opts, want) {
				t.Errorf("options %q are missing %q", opts, want)
			}
		}
		if args[i+1] == "web2" && !strings.Contains(opts, "-p 2222") {
			t.Errorf("web2:2222 without -p 2222: %q", args)
		}
		if args[i+1] == "web1" && strings.Contains(opts, "-p ") {
			t.Errorf("web1 has a port, which should be left to ssh_config: %q", args)
		}
	}
}

func TestOpenSSHAllowForwarding(t *testing.T) {
	f := newFakeSSH(t)
	cfg := f.config([]string{"web1"}, cmd("true"))
	cfg.OpenSSH.AllowForwarding = true
	cfg.OpenSSH.HostKeyChecking = ""
	if _, err := cfg.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	opts := strings.Join(f.invocations(t)[0], " ")
	if strings.Contains(opts, "ForwardAgent") || strings.Contains(opts, "StrictHostKeyChecking") {
		t.Errorf("forwarding or host key options set although left to ssh_config: %q", opts)
	}
}

func TestOpenSSHStdin(t *testing.T) {
	f := newFakeSSH(t)
	res, err := f.config([]string{"web1"}, massh.Job{Command: "cat", Stdin: []byte("piped\n")}).Run(t.Context())
	if err != nil || string(res[0].Stdout) != "piped\n" {
		t.Fatalf("Run: %v, stdout %q", err, res[0].Stdout)
	}
}

func TestOpenSSHPassword(t *testing.T) {
	f := newFakeSSH(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		password string
		ok       bool
	}{{"hunter2", true}, {"wrong", false}} {
		cfg := f.config([]string{"locked"}, cmd("echo in"))
		cfg.OpenSSH.Password = tt.password
		cfg.OpenSSH.AskpassProgram = exe
		res, err := cfg.Run(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if tt.ok && (res[0].Err != nil || string(res[0].Stdout) != "in\n") {
			t.Errorf("right password: %+v", res[0])
		}
		if !tt.ok && !strings.Contains(fmt.Sprint(res[0].Err), "Permission denied") {
			t.Errorf("wrong password: Err = %v", res[0].Err)
		}
	}
	// The password never appears in ssh's arguments.
	for _, args := range f.invocations(t) {
		if slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, "hunter2") }) {
			t.Errorf("password in arguments: %q", args)
		}
		if !slices.Contains(args, "BatchMode=no") {
			t.Errorf("BatchMode not turned off for password auth: %q", args)
		}
	}
}

func TestOpenSSHCancelSlowKillsProcessGroup(t *testing.T) {
	f := newFakeSSH(t)
	marker := filepath.Join(t.TempDir(), "still-running")
	cfg := f.config([]string{"web1"}, cmd("echo started; sleep 2; touch "+marker))
	cfg.SlowTimeout = 200 * time.Millisecond
	cfg.CancelSlow = true
	start := time.Now()
	res, err := cfg.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(res[0].Err, massh.ErrSlow) || time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("Err = %v after %v, want ErrSlow quickly", res[0].Err, time.Since(start))
	}
	// The command's children were killed too: nothing touches the marker later.
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("the remote command kept running after it was killed")
	}
}

func TestOpenSSHValidate(t *testing.T) {
	f := newFakeSSH(t)
	for _, host := range []string{"-oProxyCommand=touch /tmp/pwned", "web 1", "web\n1", "-"} {
		cfg := f.config([]string{host}, cmd("true"))
		if err := cfg.Validate(); !errors.Is(err, massh.ErrUnsafeHost) {
			t.Errorf("host %q: Validate() = %v, want ErrUnsafeHost", host, err)
		}
	}
	cfg := f.config([]string{"web1"}, cmd("true"))
	cfg.BastionHost = "-oProxyCommand=x"
	if err := cfg.Validate(); !errors.Is(err, massh.ErrUnsafeHost) {
		t.Errorf("unsafe bastion: %v", err)
	}
	cfg = f.config([]string{"web1"}, cmd("true"))
	cfg.Resolve = func(string) (massh.HostOptions, error) { return massh.HostOptions{}, nil }
	cfg.OpenSSH.Password = "x"
	cfg.OpenSSH.HostKeyChecking = "maybe"
	err := cfg.Validate()
	for _, want := range []error{massh.ErrResolveWithSSH, massh.ErrNoAskpass, massh.ErrBadHostKeyCheck} {
		if !errors.Is(err, want) {
			t.Errorf("Validate() = %v, missing %v", err, want)
		}
	}
	// SSHConfig isn't needed.
	if err := f.config([]string{"web1"}, cmd("true")).Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}

	cfg = f.config([]string{"web1"}, cmd("true"))
	cfg.OpenSSH.Path = filepath.Join(t.TempDir(), "no-such-ssh")
	if _, err := cfg.Stream(t.Context()); err == nil {
		t.Error("Stream succeeded without an ssh client")
	}
}

// TestOpenSSHReal runs the real OpenSSH client against the in-process server. It is skipped if
// ssh isn't installed.
func TestOpenSSHReal(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh client not installed")
	}
	srv, bastion, stranger := sshtest.NewServer(t), sshtest.NewServer(t), sshtest.NewServer(t)
	dir := t.TempDir()
	var known strings.Builder
	var keys []string
	for i, s := range []*sshtest.Server{srv, bastion} {
		known.WriteString(knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, s.HostKey) + "\n")
		key := filepath.Join(dir, fmt.Sprintf("key%d", i))
		if err := os.WriteFile(key, s.ClientKeyPEM, 0o600); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, "    IdentityFile "+key)
	}
	knownFile := filepath.Join(dir, "known_hosts")
	os.WriteFile(knownFile, []byte(known.String()), 0o600)
	conf := filepath.Join(dir, "config")
	os.WriteFile(conf, []byte(fmt.Sprintf("Host *\n    User %s\n    UserKnownHostsFile %s\n    IdentitiesOnly yes\n    IdentityAgent none\n%s\n",
		sshtest.User, knownFile, strings.Join(keys, "\n"))), 0o600)

	newCfg := func(hosts ...string) *massh.Config {
		return &massh.Config{
			Hosts:   hosts,
			Jobs:    []massh.Job{cmd("echo hello; echo oops >&2; exit 3")},
			OpenSSH: &massh.OpenSSH{ConfigFile: conf, HostKeyChecking: "yes", ConnectTimeout: 5 * time.Second},
		}
	}

	res, err := newCfg(srv.Addr).Run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := massh.ExitStatus(res[0].Err); status != 3 || string(res[0].Stdout) != "hello\n" || string(res[0].Stderr) != "oops\n" {
		t.Errorf("result = %+v (stderr %q)", res[0], res[0].Stderr)
	}

	// An unknown host key is rejected.
	res, _ = newCfg(stranger.Addr).Run(t.Context())
	if sshErr, ok := errors.AsType[*massh.OpenSSHError](res[0].Err); !ok || !strings.Contains(sshErr.Message, "Host key verification failed") {
		t.Errorf("unknown host: Err = %v", res[0].Err)
	}

	// Through a bastion.
	cfg := newCfg(srv.Addr)
	cfg.BastionHost = bastion.Addr
	res, _ = cfg.Run(t.Context())
	if string(res[0].Stdout) != "hello\n" || bastion.Forwards() != 1 {
		t.Errorf("via bastion: %+v, %d forwards", res[0], bastion.Forwards())
	}

	// Password authentication through the askpass helper.
	exe, _ := os.Executable()
	srv.SetPassword("hunter2")
	cfg = newCfg(srv.Addr)
	cfg.OpenSSH.Options = []string{"PreferredAuthentications=password,keyboard-interactive"}
	cfg.OpenSSH.Password, cfg.OpenSSH.AskpassProgram = "hunter2", exe
	res, _ = cfg.Run(t.Context())
	if string(res[0].Stdout) != "hello\n" {
		t.Errorf("password auth: %+v (stderr %q)", res[0], res[0].Stderr)
	}
}
