package massh

import (
	"fmt"
	"io/ioutil"
)

// Job is a single remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command    string
	Script     []byte
	ScriptArgs string
}

// SetCommand sets the Command value in Job. This is the Command executed over SSH to all hosts.
func (j *Job) SetCommand(command string) {
	j.Command = command
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
