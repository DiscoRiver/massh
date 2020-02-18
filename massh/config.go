package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

// Config is a config implementation for distributed SSH commands
type Config struct {
	Hosts 	[]string
	SSHConfig *ssh.ClientConfig
	Job *Job
	WorkerPool int
}

// Job is the remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command string
	script []byte // Unexported because we should handle retrieving the file contents.
	scriptArgs string
}

func (c *Config) SetHosts(h []string){
	c.Hosts = h
}

func (c *Config) SetSSHConfig(s *ssh.ClientConfig) {
	c.SSHConfig = s
}

func (c *Config) SetJob(j *Job) {
	c.Job = j
}

func (c *Config) SetWorkerPool(w int) {
	c.WorkerPool = w
}

func (c *Config) Run() []Result {
	return run(c)
}

func (j *Job) SetCommands(c string) {
	j.Command = c
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






