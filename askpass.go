package massh

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Environment variables that tell a program it is being run by ssh as massh's askpass helper.
const (
	askpassSocketEnv = "MASSH_ASKPASS_SOCKET"
	askpassTokenEnv  = "MASSH_ASKPASS_TOKEN"
)

// HandleAskpass answers ssh's password prompt if this process was started by ssh as massh's
// askpass helper (see OpenSSH.AskpassProgram), and then exits. Otherwise it returns at once.
// Programs that set OpenSSH.Password or OpenSSH.Passphrase must call it first thing in main.
func HandleAskpass() {
	sock, token := os.Getenv(askpassSocketEnv), os.Getenv(askpassTokenEnv)
	if sock == "" || token == "" {
		return
	}
	answer, err := askAskpassServer(sock, token, strings.Join(os.Args[1:], " "))
	if err != nil || answer == "" {
		// Refusing makes ssh treat it as a failed authentication.
		os.Exit(1)
	}
	fmt.Println(answer)
	os.Exit(0)
}

func askAskpassServer(sock, token, prompt string) (string, error) {
	conn, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	// The prompt is ssh's, so it has no newlines; strip them anyway to keep the protocol intact.
	prompt = strings.NewReplacer("\n", " ", "\r", " ").Replace(prompt)
	if _, err := fmt.Fprintf(conn, "%s\n%s\n", token, prompt); err != nil {
		return "", err
	}
	answer, err := bufio.NewReader(conn).ReadString('\n')
	return strings.TrimSuffix(answer, "\n"), err
}

// askpassServer hands secrets to askpass helpers over a Unix socket in a private directory. A
// random token, passed to the helper in its environment, proves the request comes from an ssh
// that massh started. Secrets are never put in arguments or the environment.
type askpassServer struct {
	dir, sock, token     string
	password, passphrase string
	ln                   net.Listener
}

func newAskpassServer(password, passphrase string) (*askpassServer, error) {
	dir, err := os.MkdirTemp("", "massh-askpass-")
	if err != nil {
		return nil, fmt.Errorf("massh: askpass: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("massh: askpass: %w", err)
	}
	tok := make([]byte, 32)
	rand.Read(tok)
	s := &askpassServer{
		dir:        dir,
		sock:       filepath.Join(dir, "s"),
		token:      hex.EncodeToString(tok),
		password:   password,
		passphrase: passphrase,
	}
	if s.ln, err = net.Listen("unix", s.sock); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("massh: askpass: %w", err)
	}
	go s.serve()
	return s, nil
}

// env adds what ssh needs to use program as the askpass helper to base.
func (s *askpassServer) env(base []string, program string) []string {
	env := append(slices.Clone(base),
		"SSH_ASKPASS="+program,
		"SSH_ASKPASS_REQUIRE=force", // OpenSSH 8.4+: use askpass even with a terminal
		askpassSocketEnv+"="+s.sock,
		askpassTokenEnv+"="+s.token,
	)
	// Older OpenSSH only uses askpass when DISPLAY is set.
	if !slices.ContainsFunc(base, func(kv string) bool { return strings.HasPrefix(kv, "DISPLAY=") }) {
		env = append(env, "DISPLAY=massh")
	}
	return env
}

func (s *askpassServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.answer(conn)
	}
}

func (s *askpassServer) answer(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	token, err := r.ReadString('\n')
	if err != nil || subtle.ConstantTimeCompare([]byte(strings.TrimSuffix(token, "\n")), []byte(s.token)) != 1 {
		return
	}
	prompt, _ := r.ReadString('\n')
	if answer := s.answerFor(prompt); answer != "" {
		fmt.Fprintf(conn, "%s\n", answer)
	}
}

// answerFor picks the secret for one of ssh's prompts. Anything else, such as a host key
// confirmation, gets no answer.
func (s *askpassServer) answerFor(prompt string) string {
	p := strings.ToLower(prompt)
	switch {
	case strings.Contains(p, "passphrase"):
		return s.passphrase
	case strings.Contains(p, "password"):
		return s.password
	}
	return ""
}

func (s *askpassServer) close() {
	s.ln.Close()
	os.RemoveAll(s.dir)
}
