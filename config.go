package massh

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// Config is a config implementation for distributed SSH commands
type Config struct {
	Hosts       map[string]struct{}
	SSHConfig   *ssh.ClientConfig
	Job         *Job
	JobStack    *[]Job
	WorkerPool  int
	BastionHost string
	// BastionHost's SSH config. If nil, Bastion will use SSHConfig instead.
	BastionHostSSHConfig *ssh.ClientConfig
}

// Job is a single remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command    string
	Script     []byte
	ScriptArgs string
}

// NewConfig initialises a new massh.Config.
func NewConfig() *Config {
	c := &Config{
		Hosts:                map[string]struct{}{},
		SSHConfig:            &ssh.ClientConfig{},
		BastionHostSSHConfig: &ssh.ClientConfig{},
	}
	return c
}

// SetHosts adds a slice of strings as hosts to config. Removes duplicates.
func (c *Config) SetHosts(hosts []string) {
	for i := range hosts {
		c.Hosts[hosts[i]] = struct{}{}
	}
}

// SetBastionHost sets the bastion host to use for a massh config
func (c *Config) SetBastionHost(host string) {
	c.BastionHost = host
}

// SetBastionHostConfig sets the bastion hosts's SSH client config. If value is left nil, SSHConfig will be used instead.
func (c *Config) SetBastionHostConfig(s *ssh.ClientConfig) {
	c.BastionHostSSHConfig = s
}

// SetSSHConfig sets the SSH client config for all hosts.
func (c *Config) SetSSHConfig(s *ssh.ClientConfig) {
	c.SSHConfig = s
}

// SetJob sets Job in Config.
func (c *Config) SetJob(job *Job) {
	c.Job = job
}

// SetWorkerPool populates specified number of concurrent workers in Config. It is safe for this number to be larger than the number of hosts being processed, but it must not be zero.
func (c *Config) SetWorkerPool(numWorkers int) {
	c.WorkerPool = numWorkers
}

// SetSSHAuthSockAuth uses SSH_AUTH_SOCK environment variable to populate auth method in the SSH config. Useful when using keys, and `AgentForwarding` is enabled in the local SSH config.
func (c *Config) SetSSHAuthSockAuth() error {
	// SSH_AUTH_SOCK contains the path of the unix socket that the agent uses for communication with other processes.
	SSHAuthSock, err := sshAuthSock()
	if err != nil {
		return err
	}

	c.SSHConfig.Auth = append(c.SSHConfig.Auth, SSHAuthSock)

	return nil
}

// SetSSHHostKeyCallback sets the HostKeyCallback for the Config's SSHConfig value.
func (c *Config) SetSSHHostKeyCallback(callback ssh.HostKeyCallback) {
	c.SSHConfig.HostKeyCallback = callback
}

// Run executes the config, return a slice of Results once the command has exited
func (c *Config) Run() ([]Result, error) {
	if err := checkJobs(c); err != nil {
		return nil, err
	}
	return run(c), nil
}

/*
Stream executes the config, and writes to rs as commands are initiated.

One result is added to the channel for each host. Streaming is performed by reading the StdOutStream
and StdErrStream parameters in Result.

Example reading each result in the channel:
```
cfg.Stream(resultChan)

	for {
		result := <-resultChan
		go func() {
			// do something with the result
		}()
	}
```
*/
func (c *Config) Stream(rs chan Result) error {
	if err := checkJobs(c); err != nil {
		return err
	}

	if rs == nil {
		return fmt.Errorf("stream channel cannot be nil")
	}

	runStream(c, rs)
	return nil
}

// CheckSanity ensures config is valid.
func (c *Config) CheckSanity() error {
	if err := checkConfigSanity(c); err != nil {
		return err
	}
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

// SetCommand sets the Command value in Job. This is the Command executed over SSH to all hosts.
func (j *Job) SetCommand(command string) {
	j.Command = command
}

// SetPasswordAuth sets ssh password from provided byte slice (read from terminal)
func (c *Config) SetPasswordAuth(username string, password string) {
	c.SSHConfig.User = username
	c.SSHConfig.Auth = append(c.SSHConfig.Auth, ssh.Password(password))
}

// SetLocalScript reads a script file contents into the Job config.
func (j *Job) SetLocalScript(filename string, args string) error {
	var err error
	j.Script, err = ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read script file")
	}
	j.ScriptArgs = args

	return nil
}
