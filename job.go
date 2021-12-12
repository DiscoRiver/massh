package massh

// Job is a single remote task config. For script files, use Job.SetLocalScript().
type Job struct {
	Command string
	Script  script
}

// SetCommand sets the Command value in Job. This is the Command executed over SSH to all hosts.
func (j *Job) SetCommand(command string) {
	j.Command = command
}

func (j *Job) SetScript(filePath string, args ...string) error {
	s, err := newScript(filePath, args...)
	if err != nil {
		return err
	}

	j.Script = s

	return nil
}
