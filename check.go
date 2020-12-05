package massh

import "fmt"

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
	if c.WorkerPool == 0 {
		e = append(e, "WorkerPool")
	}

	if e != nil {
		return fmt.Errorf("sanity check failed, the following config values are not set: %s", e[0:])
	}
	return nil
}
