package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

// Config is a config implementation for distributed SSH commands
type Config struct {
	Hosts      []string
	SSHConfig  *ssh.ClientConfig
	Job        *Job
	WorkerPool int
}

// Job is the remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command    string
	script     []byte // Unexported because we should handle this internally
	scriptArgs string
}

// SetHosts adds a slice of strings as hosts to config
func (c *Config) SetHosts(hosts []string) {
	c.Hosts = hosts
}

func (c *Config) SetSSHConfig(s *ssh.ClientConfig) {
	c.SSHConfig = s
}

func (c *Config) SetJob(job *Job) {
	c.Job = job
}

// SetWorkerPool populates specified number of concurrent workers in Config.
func (c *Config) SetWorkerPool(numWorkers int) {
	c.WorkerPool = numWorkers
}

// SetSSHAuthSockAuth uses SSH_AUTH_SOCK environment variable to populate auth method in the SSH config.
func (c *Config) SetSSHAuthSockAuth() {
	// SSH_AUTH_SOCK contains the path of the unix socket that the agent uses for communication with other processes.
	if SSHAuthSock, err := getSSHAuthSock(); err == nil {
		c.SSHConfig.Auth = append(c.SSHConfig.Auth, SSHAuthSock)
	}
}

// Run executes the config, return a slice of Results.
func (c *Config) Run() ([]Result, error) {
	return run(c), nil
}

func (c *Config) CheckSanity() error {
	if err := checkConfigSanity(c); err != nil {
		return err
	}
	return nil
}

// SetKeySignature takes the file provided, reads it, and adds the key signature to the config.
func (c *Config) SetPublicKeyAuth(PublicKeyFile string) error {
	// read private key file
	key, err := ioutil.ReadFile(PublicKeyFile)
	if err != nil {
		return fmt.Errorf("unable to read public key file: %s", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("unable to parse public key: %s", err)
	}

	c.SSHConfig.Auth = []ssh.AuthMethod{
		ssh.PublicKeys(signer),
	}

	return nil
}

func (j *Job) SetCommand(command string) {
	j.Command = command
}

// SetPasswordAuth sets ssh password from provided byte slice (read from terminal)
func (c *Config) SetPasswordAuth(password []byte) error {
	c.SSHConfig.Auth = []ssh.AuthMethod{
		ssh.Password(string(password)),
	}

	return nil
}

// SetLocalScript reads a script file contents into the Job config.
func (j *Job) SetLocalScript(s string, args string) error {
	var err error
	j.script, err = ioutil.ReadFile(s)
	if err != nil {
		return fmt.Errorf("failed to read script file")
	}
	j.scriptArgs = args

	return nil
}
