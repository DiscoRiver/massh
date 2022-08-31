// Package massh provides tools for running distributed shell commands via SSH.
//
// A Massh config should be configured with the minimum of a Job, Hosts, and an SSHConfig, followed by a call to
// either Run() or Stream(), depending on your requirements.
package massh

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// Config is a collection of parameters for running distributed SSH commands. A new config should always be generated
// using NewConfig.
type Config struct {
	Hosts     map[string]struct{}
	SSHConfig *ssh.ClientConfig

	// Jobs to execute, config will error if both are set
	Job      *Job
	JobStack *[]Job

	// Number of concurrent workers
	WorkerPool int

	BastionHost string
	// BastionHost's SSH config. If nil, Bastion will use SSHConfig instead.
	BastionHostSSHConfig *ssh.ClientConfig

	// Stream-only
	SlowTimeout     int  // Timeout for delcaring that a host is slow.
	CancelSlowHosts bool // Not implemented. Automatically cancel hosts that are flagged as slow.
	Stop            chan struct{}
}

// NewConfig initialises a new Config.
func NewConfig() *Config {
	return &Config{
		Hosts:                map[string]struct{}{},
		SSHConfig:            &ssh.ClientConfig{},
		BastionHostSSHConfig: &ssh.ClientConfig{},
		Stop:                 make(chan struct{}, 1),
	}
}

// Run returns a slice of results once every host has completed it's work, successful or otherwise.
//
// This is a rudimentary function designed for small, simple jobs that require low complexity. As such, it's
// execution is not affected by SlowTimeout or CancelSlowHosts. The Results returned using this method
// always have an IsSlow value of false.
func (c *Config) Run() ([]Result, error) {
	if err := checkJobs(c); err != nil {
		return nil, err
	}
	return run(c), nil
}

/*
Stream populates rs as commands are initiated, and writes each host's output concurrently. It allows
real-time output processing and premature cancellation.

Stdout and Stderr can be read from StdOutStream and StdErrStream respectively.

Example for reading each result in the channel:
```
resultChan := make(chan *Result)
cfg.Stream(resultChan)
	for {
		result := <-resultChan
		go func() {
			// do something with the result
		}()
	}
```

More complete examples can be found in test files or in _examples.
*/
func (c *Config) Stream(rs chan *Result) error {
	if err := checkJobs(c); err != nil {
		return err
	}

	if rs == nil {
		return fmt.Errorf("stream channel cannot be nil")
	}

	runStream(c, rs)
	return nil
}

// SetPrivateKeyAuth takes the private key file provided, reads it, and adds the key signature to the config.
func (c *Config) SetPrivateKeyAuth(PrivateKeyFile string, PrivateKeyPassphrase string) error {
	if strings.HasPrefix(PrivateKeyFile, "~/") {
		home, _ := homedir.Dir()
		PrivateKeyFile = filepath.Join(home, PrivateKeyFile[2:])
	}

	key, err := ioutil.ReadFile(PrivateKeyFile)
	if err != nil {
		return fmt.Errorf("unable to read private key file: %s", err)
	}

	// Create the Signer for this private key.
	var signer ssh.Signer
	if PrivateKeyPassphrase == "" {
		var err error
		signer, err = ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %s", err)
		}
	} else {
		var err error
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(PrivateKeyPassphrase))
		if err != nil {
			return fmt.Errorf("unable to parse private key with passphrase: %s", err)
		}
	}

	c.SSHConfig.Auth = append(c.SSHConfig.Auth, ssh.PublicKeys(signer))

	return nil
}

// SetPasswordAuth sets ssh password from provided byte slice (read from terminal)
func (c *Config) SetPasswordAuth(username string, password string) {
	c.SSHConfig.User = username
	c.SSHConfig.Auth = append(c.SSHConfig.Auth, ssh.Password(password))
}

// SetSlowTimeout sets the SlowTimeout value for config.
func (c *Config) SetSlowTimeout(timeout int) {
	c.SlowTimeout = timeout
}

// SetHosts adds a slice of strings as hosts to config. It will filter out duplicate hosts.
func (c *Config) SetHosts(hosts []string) {
	for i := range hosts {
		c.Hosts[hosts[i]] = struct{}{}
	}
}

// SetBastionHost sets the bastion host for config.
func (c *Config) SetBastionHost(host string) {
	c.BastionHost = host
}

// SetBastionHostConfig sets the bastion hosts's SSH client config. If value is left nil, SSHConfig will be used instead.
func (c *Config) SetBastionHostConfig(s *ssh.ClientConfig) {
	c.BastionHostSSHConfig = s
}

// SetSSHConfig set the SSHConfig for config.
func (c *Config) SetSSHConfig(s *ssh.ClientConfig) {
	c.SSHConfig = s
}

// SetJob sets Job for config.
func (c *Config) SetJob(job *Job) {
	c.Job = job
}

// SetWorkerPool specifies the number of concurrent workers for config. If numWorkers is less than 1, it will be
// set to 1 instead.
func (c *Config) SetWorkerPool(numWorkers int) {
	if numWorkers < 1 {
		c.WorkerPool = 1
		return
	}
	c.WorkerPool = numWorkers
}

// SetSSHAuthSock uses SSH_AUTH_SOCK environment variable to populate auth method in the SSHConfig.
//
// This is useful when using keys, and `AgentForwarding` is enabled in the client's SSH config.
func (c *Config) SetSSHAuthSock() error {
	// SSH_AUTH_SOCK contains the path of the unix socket that the agent uses for communication with other processes.
	SSHAuthSock, err := sshAuthSock()
	if err != nil {
		return err
	}

	c.SSHConfig.Auth = append(c.SSHConfig.Auth, SSHAuthSock)

	return nil
}

// SetSSHHostKeyCallback sets the HostKeyCallback for the Config's SSHConfig value.
//
// This value should not be set to ssh.InsecureIgnoreHostKey() in production!
func (c *Config) SetSSHHostKeyCallback(callback ssh.HostKeyCallback) {
	c.SSHConfig.HostKeyCallback = callback
}

// StopAllSessions stops all active streaming jobs.
func (c *Config) StopAllSessions() {
	c.Stop <- struct{}{}
}

// CheckSanity ensures config is valid.
func (c *Config) CheckSanity() error {
	if err := checkConfigSanity(c); err != nil {
		return err
	}
	return nil
}

// AutoCancelSlowHosts will cancel/terminate slow host sessions.
func (c *Config) AutoCancelSlowHosts() {
	c.CancelSlowHosts = true
}
