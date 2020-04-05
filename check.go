package massh

import "fmt"

func checkConfigSanity(c *Config) error {
	var e []string
	if c.Hosts == nil {
		e = append(e, "Hosts")
	} else if c.Job == nil {
		e = append(e, "Job")
	} else if c.SSHConfig == nil {
		e = append(e, "SSHConfig")
	} else if c.WorkerPool == 0 {
		e = append(e, "WorkerPool")
	}

	if e != nil {
		return fmt.Errorf("sanity check failed, the following config values are not set: %s", e[0:])
	}
	return nil
}
