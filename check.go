package massh

import (
	"errors"
	"fmt"
)

var (
	ErrJobConflict = errors.New("only one of job or jobstack must be present in config")
	ErrNoJobsSet = errors.New("no jobs are set in config")
)

// TODO: Make this more robust, and automatically performed when running config.Run or config.Stream.
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
		return fmt.Errorf("bad config, the following config items are not correct: %s", e[0:])
	}
	return nil
}

func checkJobs(c *Config) error {
	if c.Job != nil && c.JobStack != nil {
		return ErrJobConflict
	} else if c.Job == nil && c.JobStack == nil {
		return ErrNoJobsSet
	}
	return nil
}
