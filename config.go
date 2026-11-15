package massh

import (
	"errors"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// DefaultPort is the SSH port used for hosts that don't specify one, unless Config.Port is set.
	DefaultPort = 22

	// DefaultWorkers is the number of hosts processed concurrently when Config.Workers is not set.
	DefaultWorkers = 32
)

// Errors reported by [Config.Validate].
var (
	ErrNoHosts           = errors.New("massh: no hosts")
	ErrEmptyHost         = errors.New("massh: empty host name")
	ErrNoJobs            = errors.New("massh: no jobs")
	ErrNoSSHConfig       = errors.New("massh: SSHConfig is nil")
	ErrNoAuth            = errors.New("massh: SSHConfig has no auth methods")
	ErrNoHostKeyCallback = errors.New("massh: SSHConfig has no HostKeyCallback")
)

// Config describes work to run on a set of hosts. The zero value is not runnable: at least Hosts,
// Jobs and SSHConfig must be set. A Config must not be modified while Run or Stream is in progress.
type Config struct {
	// Hosts to run Jobs on. Each is a hostname or IP address, optionally with a port
	// ("example.com:2222", "[::1]:2222"). Duplicates are ignored.
	Hosts []string

	// Jobs run in order on every host, each in its own SSH session.
	Jobs []Job

	// SSHConfig is used to connect to every host. It must have at least one auth method and a
	// HostKeyCallback. Its Timeout bounds both the TCP connection and the SSH handshake.
	SSHConfig *ssh.ClientConfig

	// Port is used for hosts that don't include one. Zero means DefaultPort.
	Port int

	// BastionHost, if set, is a jump host that every connection is tunnelled through, in the same
	// format as Hosts. A single connection to it is shared by all hosts.
	BastionHost string

	// BastionSSHConfig is used to connect to BastionHost. Nil means SSHConfig.
	BastionSSHConfig *ssh.ClientConfig

	// Workers is the maximum number of hosts processed concurrently. Zero means DefaultWorkers.
	Workers int

	// SlowTimeout, if positive, flags a job as slow when it has produced no output for this long.
	// An EventSlow event is sent and Result.Slow is set.
	SlowTimeout time.Duration

	// CancelSlow stops jobs as soon as they are flagged slow. Their error is ErrSlow.
	CancelSlow bool

	// Resolve, if set, is called for each host to override how it is reached, for example
	// from ~/.ssh/config. If it returns an error, that host fails with it.
	Resolve func(host string) (HostOptions, error)
}

// HostOptions overrides how one host is reached. Zero fields keep the Config's settings.
type HostOptions struct {
	// Addr is the address to dial instead of the host name, with or without a port.
	Addr string
	// SSHConfig is used instead of Config.SSHConfig.
	SSHConfig *ssh.ClientConfig
	// Bastion is a jump host to use instead of Config.BastionHost.
	Bastion string
	// BastionSSHConfig is used to connect to Bastion. Nil means Config.BastionSSHConfig if
	// Bastion is Config.BastionHost, and the host's SSH config otherwise.
	BastionSSHConfig *ssh.ClientConfig
}

// Validate reports every problem that would prevent c from running, joined into one error.
func (c *Config) Validate() error {
	var errs []error
	if len(c.Hosts) == 0 {
		errs = append(errs, ErrNoHosts)
	}
	// An empty host would silently mean the local machine.
	if slices.ContainsFunc(c.Hosts, func(h string) bool { return strings.TrimSpace(h) == "" }) {
		errs = append(errs, ErrEmptyHost)
	}
	if len(c.Jobs) == 0 {
		errs = append(errs, ErrNoJobs)
	}
	errs = append(errs, validateSSHConfig(c.SSHConfig))
	if c.BastionHost != "" && c.BastionSSHConfig != nil {
		errs = append(errs, validateSSHConfig(c.BastionSSHConfig))
	}
	return errors.Join(errs...)
}

func validateSSHConfig(s *ssh.ClientConfig) error {
	if s == nil {
		return ErrNoSSHConfig
	}
	var errs []error
	if len(s.Auth) == 0 {
		errs = append(errs, ErrNoAuth)
	}
	if s.HostKeyCallback == nil {
		errs = append(errs, ErrNoHostKeyCallback)
	}
	return errors.Join(errs...)
}

// uniqueHosts returns c.Hosts without duplicates, in their original order.
func (c *Config) uniqueHosts() []string {
	seen := make(map[string]struct{}, len(c.Hosts))
	hosts := make([]string, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	return slices.Clip(hosts)
}

func (c *Config) port() int {
	if c.Port > 0 {
		return c.Port
	}
	return DefaultPort
}

func (c *Config) workers() int {
	if c.Workers > 0 {
		return c.Workers
	}
	return DefaultWorkers
}

func (c *Config) bastionSSHConfig() *ssh.ClientConfig {
	if c.BastionSSHConfig != nil {
		return c.BastionSSHConfig
	}
	return c.SSHConfig
}
