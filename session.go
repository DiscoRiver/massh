package massh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
)

var (
	// NumberOfStreamingHostsCompleted is incremented when a Result's DoneChannel is written to, indicating a host has completed it's work.
	NumberOfStreamingHostsCompleted int
)

const (
	sshPort = "22"
)

// Result contains usable output from SSH commands.
type Result struct {
	Host   string // Hostname
	Job    string // The command that was run
	Output []byte

	// Package errors, not output from SSH. Makes the concurrency easier to manage without returning an error.
	Error error

	// Stream-specific
	StdOutStream chan []byte
	StdErrStream chan []byte
	// Written to when a host completes it's work. This does not indicate that all output from StdOutStream or StdErrStream has been read and/or processed.
	DoneChannel chan struct{}
}

// sshCommand runs an SSH task and returns Result only when the command has finished executing.
func sshCommand(host string, config *Config) Result {
	var r Result

	// Never return a Result with a blank host
	r.Host = host

	client, err := generateSSHClientWithPotentialBastion(host, config)
	if err != nil {
		r.Error = err
		return r
	}
	defer client.Close()

	session, err := newClientSession(client)
	if err != nil {
		r.Error = fmt.Errorf("failed to create session: %v", err)
		return r
	}
	defer session.Close()

	// Get job string
	job := getJob(session, config.Job)
	r.Job = job

	// run the job
	var b bytes.Buffer
	session.Stdout = &b
	if err := runJob(session, job); err != nil {
		r.Error = err
		return r
	}

	r.Output = b.Bytes()

	return r
}

func sshCommandStream(host string, config *Config, resultChannel chan Result) {
	var r Result
	// This is needed so we don't need to write to the channel before every return statement when erroring..
	defer func() {
		if r.Error != nil {
			resultChannel <- r
			NumberOfStreamingHostsCompleted++
		} else {
			r.DoneChannel <- struct{}{}
		}
	}()

	// Never send to the result channel with a blank host.
	r.Host = host

	client, err := generateSSHClientWithPotentialBastion(host, config)
	if err != nil {
		r.Error = err
		return
	}
	defer client.Close()

	session, err := newClientSession(client)
	if err != nil {
		r.Error = fmt.Errorf("failed to create session: %s", err)
		return
	}
	defer session.Close()

	// Get job string
	job := getJob(session, config.Job)
	r.Job = job

	// Set the stdout pipe which we will read/redirect later to our stdout channel
	StdOutPipe, err := session.StdoutPipe()
	if err != nil {
		r.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
		return
	}
	// Channel used for streaming stdout
	stdout := make(chan []byte)
	r.StdOutStream = stdout

	// Set the stderr pipe which we will read/redirect later to our stderr channel
	StdErrPipe, err := session.StderrPipe()
	if err != nil {
		r.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
		return
	}
	// Channel used for streaming stderr
	stderr := make(chan []byte)
	r.StdErrStream = stderr

	// Set up a special channel to report completion of the ssh task. This is easier than handling exit codes etc.
	//
	// Using struct{} for memory saving as it takes up 0 bytes; bool take up 1, and we don't actually care
	// what is written to the done channel, just that "something" is read from it so that we know the
	// command exited.
	done := make(chan struct{})
	r.DoneChannel = done

	// Reading from our pipes as they're populated, and redirecting bytes to our stdout and stderr channels in Result.
	//
	// We're doing this before we start the ssh task so we can start churning through output as soon as it starts.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		readToBytesChannel(StdOutPipe, r.StdOutStream, r, &wg)
		readToBytesChannel(StdErrPipe, r.StdErrStream, r, &wg)
	}()

	resultChannel <- r

	// Start the job immediately, but don't wait for the command to exit
	if err := startJob(session, job); err != nil {
		r.Error = fmt.Errorf("could not start job: %s", err)
		return
	}

	// Wait for the command to exit only after we've initiated all the output channels
	wg.Wait()
	session.Wait()

	NumberOfStreamingHostsCompleted++
}

// readToBytesChannel reads from io.Reader and directs the data to a byte slice channel for streaming.
func readToBytesChannel(reader io.Reader, stream chan []byte, r Result, wg *sync.WaitGroup) {
	defer func(){ wg.Done() }()

	buf := bufio.NewReader(reader)

	for {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return
			} else {
				r.Error = fmt.Errorf("couldn't read content to stream channel: %s", err)
				return
			}
		}

		stream <- line

	}
}

// worker invokes sshCommand for each host in the channel
func worker(hosts <-chan string, results chan<- Result, config *Config, resChan chan Result) {
	if resChan == nil {
		for host := range hosts {
			cfg := *config
			if cfg.JobStack != nil {
				for i := range *cfg.JobStack {
					j := (*cfg.JobStack)[i]
					cfg.Job = &j
					results <- sshCommand(host, &cfg)
				}
			} else {
				results <- sshCommand(host, config)
			}
		}
	} else {
		for host := range hosts {
			cfg := *config
			if cfg.JobStack != nil {
				for i := range *cfg.JobStack {
					j := (*cfg.JobStack)[i]
					cfg.Job = &j
					sshCommandStream(host, &cfg, resChan)
				}
			} else {
				sshCommandStream(host, config, resChan)
			}
		}
	}
}

// runStream is mostly the same as run, except it direct the results to a channel so they can be processed
// before the command has completed executing (i.e streaming the stdout and stderr as it runs).
func runStream(c *Config, rs chan Result) {
	// Channels length is always how many hosts we have multiplied by the number of jobs we're running.
	var resultChanLength int
	if c.JobStack != nil {
		resultChanLength = len(c.Hosts) * len(*c.JobStack)
	} else {
		resultChanLength = len(c.Hosts)
	}
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, resultChanLength)

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c, rs)
	}

	// This is what actually triggers the worker(s) to trigger. Each workers takes a host, and when it becomes
	// available again, it will take another host as long as there are host to be received.
	for k := range c.Hosts {
		hosts <- k // send each host to the channel
	}
	// Indicate nothing more will be written
	close(hosts)
}

// run sets up goroutines, worker pool, and returns the command results.
func run(c *Config) (res []Result) {
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c, nil)
	}

	for k := range c.Hosts {
		hosts <- k // send each host to the channel
	}
	close(hosts)

	for r := 0; r < len(c.Hosts); r++ {
		res = append(res, <-results)
	}

	return res
}
