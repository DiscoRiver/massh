package massh

import "fmt"

// TODO: Make this required when running Run or Stream. It's currently on the user.
func checkConfigSanity(c *Config) error {
	var e []string
	if c.Hosts == nil {
		e = append(e, "Hosts")
	}
	if c.Job == nil {
		e = append(e, "Job")
	}
	if c.SSHConfig == nil {
		e = append(e, "SSHConfig")
	}
	// not setting a worker pool results program hanging forever.
	if c.WorkerPool == 0 {
		e = append(e, "WorkerPool")
	}

	if e != nil {
		return fmt.Errorf("sanity check failed, the following config values are not set: %s", e[0:])
	}
	return nil
}
