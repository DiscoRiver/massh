package massh

import "fmt"

// TODO: Make this required when running Run or Stream. It's currently on the user.
func checkConfigSanity(c *Config) error {
	var e []string
	if c.Hosts == nil {
		e = append(e, "Hosts")
	}
	if c.Job == nil && c.JobStack == nil{
		e = append(e, "Jobs")
	}
	if c.SSHConfig == nil {
		e = append(e, "SSHConfig")
	}
	// not setting a worker pool results program hanging forever.
	if c.WorkerPool == 0 {
		e = append(e, "WorkerPool")
	}

	if e != nil {
		return fmt.Errorf("sanity check failed, the following config items are not correct: %s", e[0:])
	}
	return nil
}

func checkJobs(c *Config) error {
	if c.Job != nil && c.JobStack != nil {
		return fmt.Errorf("both Job and JobStack cannot be present in config, use Job for single command, and JobStack for multiple commands")
	} else if c.Job == nil && c.JobStack == nil {
		return fmt.Errorf("no jobs present in config")
	}
	return nil
}
