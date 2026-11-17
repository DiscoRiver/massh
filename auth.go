package massh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// PrivateKeySigner reads the private key in the file at path, decrypting it with passphrase if
// that is not empty. A leading "~/" in path is replaced by the user's home directory.
func PrivateKeySigner(path, passphrase string) (ssh.Signer, error) {
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
	return signer, nil
}

// PrivateKeyAuth returns an auth method that uses the private key file at path. See
// [PrivateKeySigner].
//
// The SSH client tries each kind of auth method only once, so a second "publickey" method in
// ssh.ClientConfig.Auth (such as [AgentAuth]) is skipped if the first is rejected. To offer keys
// from several sources, combine them in one [ssh.PublicKeysCallback], using [PrivateKeySigner]
// and [AgentSigners].
func PrivateKeyAuth(path, passphrase string) (ssh.AuthMethod, error) {
	signer, err := PrivateKeySigner(path, passphrase)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// AgentSigners connects to the SSH agent listening on $SSH_AUTH_SOCK and returns a function that
// lists the agent's keys, for use with [ssh.PublicKeysCallback]. The connection to the agent stays
// open for the life of the process.
func AgentSigners() (func() ([]ssh.Signer, error), error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("massh: SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("massh: connect to SSH agent: %w", err)
	}
	return agent.NewClient(conn).Signers, nil
}

// AgentAuth returns an auth method that uses the keys held by the SSH agent. See [AgentSigners]
// and the note on [PrivateKeyAuth].
func AgentAuth() (ssh.AuthMethod, error) {
	signers, err := AgentSigners()
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(signers), nil
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

// KnownHostsAcceptNew is like KnownHosts for a single file, except that keys of hosts that
// aren't in the file yet are accepted and added to it, like OpenSSH's
// "StrictHostKeyChecking accept-new". A host whose key has changed is still rejected. The file is
// created if it doesn't exist.
func KnownHostsAcceptNew(file string) (ssh.HostKeyCallback, error) {
	file, err := expandHome(file)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return nil, fmt.Errorf("massh: known hosts: %w", err)
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("massh: known hosts: %w", err)
	}
	f.Close()

	var mu sync.Mutex
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		mu.Lock()
		defer mu.Unlock()
		// Re-read the file each time, so that keys added for other hosts are seen.
		check, err := knownhosts.New(file)
		if err != nil {
			return fmt.Errorf("massh: known hosts: %w", err)
		}
		err = check(hostname, remote, key)
		keyErr, ok := errors.AsType[*knownhosts.KeyError](err)
		if !ok || len(keyErr.Want) > 0 {
			// Known and matching (nil), changed key, or another problem.
			return err
		}
		f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("massh: known hosts: %w", err)
		}
		defer f.Close()
		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("massh: known hosts: %w", err)
		}
		return nil
	}, nil
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
