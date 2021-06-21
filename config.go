package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

const (
	preProcessWorkingDirectoryEnv = "MASSH_WORK_ENV"
)

// Config is a config implementation for distributed SSH commands
type Config struct {
	Hosts      []string
	SSHConfig  *ssh.ClientConfig
	Job        *Job
	WorkerPool int
	BastionHost string
	// If nil, will use SSHConfig.
	BastionHostSSHConfig *ssh.ClientConfig
}

// Job is the remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command    string
	Script     []byte
	ScriptArgs string
	PreProcessScript []byte
	PreProcessScriptArgs string
}

// SetHosts adds a slice of strings as hosts to config
func (c *Config) SetHosts(hosts []string) {
	c.Hosts = hosts
}

// SetBastionHost sets the bastion host to use for a massh config
func (c *Config) SetBastionHost(host string) {
	c.BastionHost = host
}

func (c *Config) SetBastionHostConfig(s *ssh.ClientConfig) {
	c.BastionHostSSHConfig = s
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
	if SSHAuthSock, err := sshAuthSock(); err == nil {
		c.SSHConfig.Auth = append(c.SSHConfig.Auth, SSHAuthSock)
	}
}

// Run executes the config, return a slice of Results once the command has exited
func (c *Config) Run() ([]Result, error) {
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
	if rs == nil {
		return fmt.Errorf("stream channel cannot be nil")
	}
	runStream(c, rs)
	return nil
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
func (j *Job) SetLocalScript(filename string, args string) error {
	var err error
	j.Script, err = ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read script file")
	}
	j.ScriptArgs = args

	return nil
}

// SetPreProcessingScript reads in the file and args, and adds it to the Job. This script should return the environment
// variable MASSH_WORK_ENV. Failure to access this variable if a pre-processing script is present will result in the command
// to fail.
func (j *Job) SetPreProcessingScript(filename string, args string) error {
	var err error
	j.PreProcessScript, err = ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read script file")
	}
	j.PreProcessScriptArgs = args

	return nil
}


