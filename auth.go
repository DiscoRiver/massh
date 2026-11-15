package massh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// PrivateKeyAuth returns an auth method that uses the private key in the file at path, decrypting
// it with passphrase if that is not empty. A leading "~/" in path is replaced by the user's home
// directory.
func PrivateKeyAuth(path, passphrase string) (ssh.AuthMethod, error) {
	path, err := expandHome(path)
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("massh: read private key: %w", err)
	}

	var signer ssh.Signer
	if passphrase == "" {
		signer, err = ssh.ParsePrivateKey(key)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
	}
	if err != nil {
		return nil, fmt.Errorf("massh: parse private key %s: %w", path, err)
	}
	return ssh.PublicKeys(signer), nil
}

// AgentAuth returns an auth method that uses the keys held by the SSH agent listening on
// $SSH_AUTH_SOCK. The connection to the agent stays open for the life of the process.
func AgentAuth() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("massh: SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("massh: connect to SSH agent: %w", err)
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}

// KnownHosts returns a host key callback that accepts only the keys listed in the given
// known_hosts files. With no files, it uses ~/.ssh/known_hosts. A leading "~/" in a file name is
// replaced by the user's home directory.
func KnownHosts(files ...string) (ssh.HostKeyCallback, error) {
	if len(files) == 0 {
		files = []string{"~/.ssh/known_hosts"}
	}
	expanded := make([]string, len(files))
	for i, f := range files {
		var err error
		if expanded[i], err = expandHome(f); err != nil {
			return nil, err
		}
	}
	cb, err := knownhosts.New(expanded...)
	if err != nil {
		return nil, fmt.Errorf("massh: known hosts: %w", err)
	}
	return cb, nil
}

func expandHome(path string) (string, error) {
	rest, ok := strings.CutPrefix(path, "~/")
	if !ok {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("massh: expand %s: %w", path, err)
	}
	return filepath.Join(home, rest), nil
}
