package massh

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"sync"
	"time"
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
	IsSlow       bool // Activity timeout for StdOut
	StdOutStream chan []byte
	StdErrStream chan []byte
	DoneChannel  chan struct{} // Written to when a host completes work. This does not indicate that all output from StdOutStream or StdErrStream has been read and/or processed.
}

// getJob determines the type of job and returns the command string. If type is a local script, then stdin will be populated with the script data and sent/executed on the remote machine.
func getJob(s *ssh.Session, j *Job) string {
	// Set up remote script
	if j.Script != nil {
		j.Script.prepare(s)
		return j.Script.getPreparedCommandString()
	}

	return j.Command
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
	r.Job = getJob(session, config.Job)

	// run the job
	var b bytes.Buffer
	session.Stdout = &b
	if err := runJob(session, r.Job); err != nil {
		r.Error = err
		return r
	}

	r.Output = b.Bytes()

	return r
}

func sshCommandStream(host string, config *Config, resultChannel chan *Result) {
	streamResult := &Result{}
	// This is needed so we don't need to write to the channel before every return statement when erroring..
	defer func() {
		if streamResult.Error != nil {
			resultChannel <- streamResult
			NumberOfStreamingHostsCompleted++
		} else {
			streamResult.DoneChannel <- struct{}{}
		}
	}()

	// Never send to the result channel with a blank host.
	streamResult.Host = host

	client, err := generateSSHClientWithPotentialBastion(host, config)
	if err != nil {
		streamResult.Error = err
		return
	}
	defer client.Close()

	session, err := newClientSession(client)
	if err != nil {
		streamResult.Error = fmt.Errorf("failed to create session: %s", err)
		return
	}
	defer session.Close()

	// Get job string
	streamResult.Job = getJob(session, config.Job)

	// Set the stdout pipe which we will read/redirect later to our stdout channel
	StdOutPipe, err := session.StdoutPipe()
	if err != nil {
		streamResult.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
		return
	}
	// Channel used for streaming stdout
	streamResult.StdOutStream = make(chan []byte)

	// Set the stderr pipe which we will read/redirect later to our stderr channel
	StdErrPipe, err := session.StderrPipe()
	if err != nil {
		streamResult.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
		return
	}
	// Channel used for streaming stderr
	streamResult.StdErrStream = make(chan []byte)

	// Set up a special channel to report completion of the ssh task. This is easier than handling exit codes etc.
	//
	// Using struct{} for memory saving as it takes up 0 bytes; bool take up 1, and we don't actually care
	// what is written to the done channel, just that "something" is read from it so that we know the
	// command exited.
	streamResult.DoneChannel = make(chan struct{})

	// Reading from our pipes as they're populated, and redirecting bytes to our stdout and stderr channels in Result.
	//
	// We're doing this before we start the ssh task so we can start churning through output as soon as it starts.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		readToBytesChannel(StdOutPipe, streamResult.StdOutStream, streamResult, config.SlowTimeout, &wg)
		readToBytesChannel(StdErrPipe, streamResult.StdErrStream, streamResult, config.SlowTimeout, &wg)
	}()

	resultChannel <- streamResult

	// Start the job immediately, but don't wait for the command to exit.
	//
	// Currently, will hang if a host fails to connect, in which case the SSHTimeout value is how long it takes for this func to return.
	if err := startJob(session, streamResult.Job); err != nil {
		streamResult.Error = fmt.Errorf("could not start job: %s", err)
		return
	}

	go func() {
		select {
		case <-config.Stop:
			session.Close()
		}
	}()

	// Wait for the command to exit only after we've initiated all the output channels
	wg.Wait()
	session.Wait()

	NumberOfStreamingHostsCompleted++
}

// readToBytesChannel reads from io.Reader and directs the data to a byte slice channel for streaming.
func readToBytesChannel(reader io.Reader, stream chan []byte, r *Result, slowTimeout int, wg *sync.WaitGroup) {
	defer func() { wg.Done() }()

	slowTimeoutDuration := time.Duration(slowTimeout) * time.Second
	t := time.NewTimer(slowTimeoutDuration)

	go func() {
		for {
			select {
			case <-t.C:
				t.Stop()
				r.IsSlow = true
				break
			}
		}
	}()

	rdr := bufio.NewReader(reader)
	for {
		line, err := rdr.ReadBytes('\n') // ReadBytes will wait until new line character is read.
		t.Reset(slowTimeoutDuration)
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
func worker(hosts <-chan string, results chan<- Result, config *Config, resChan chan *Result) {
	// This check to determine Run vs. Stream is safe because massh.Config.Stream() will not allow work to be done if it's channel
	// parameter is nil, so we only get a nil resChan when using massh.Config.Run().
	//
	// TODO: Make the handling of a JobStack more elegant.
	if resChan == nil {
		for host := range hosts {
			if config.JobStack != nil {
				for i := range *config.JobStack {
					// Cfg is a copy of config, without job pointers. This is needed to separate the jobstack.
					cfg := copyConfigNoJobs(config)

					j := (*config.JobStack)[i]
					cfg.Job = &j

					results <- sshCommand(host, cfg)
				}
			} else {
				results <- sshCommand(host, config)
			}
		}
	} else {
		for host := range hosts {
			if config.JobStack != nil {
				for i := range *config.JobStack {
					// Cfg is a copy of config, without job pointers. This is needed to separate the jobstack.
					cfg := copyConfigNoJobs(config)

					j := (*config.JobStack)[i]
					cfg.Job = &j

					go sshCommandStream(host, cfg, resChan)
				}
			} else {
				go sshCommandStream(host, config, resChan)
			}
		}
	}
}

func copyConfigNoJobs(config *Config) *Config {
	return &Config{
		Hosts:                config.Hosts,
		SSHConfig:            config.SSHConfig,
		BastionHost:          config.BastionHost,
		BastionHostSSHConfig: config.BastionHostSSHConfig,
		WorkerPool:           config.WorkerPool,
	}
}

// runStream is mostly the same as run, except it directs the results to a channel so they can be processed
// before the command has completed executing (i.e streaming the stdout and stderr as it runs).
func runStream(c *Config, rs chan *Result) {
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

	// This is what actually triggers the worker(s). Each workers takes a host, and when it becomes
	// available again, it will take another host as long as there are host to be received.
	for k := range c.Hosts {
		hosts <- k // send each host to the channel
	}
	// Indicate nothing more will be written
	close(hosts)
}

// run sets up goroutines, worker pool, and returns the command results for all hosts as a slice of Result. This can cause
// excessive memory usage if returning a large amount of data for a large number of hosts.
func run(c *Config) (res []Result) {
	// Channels length is always how many hosts we have multiplied by the number of jobs we're running.
	var resultChanLength int
	if c.JobStack != nil {
		resultChanLength = len(c.Hosts) * len(*c.JobStack)
	} else {
		resultChanLength = len(c.Hosts)
	}
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, resultChanLength)

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c, nil)
	}

	for k := range c.Hosts {
		hosts <- k // send each host to the channel
	}
	close(hosts)

	for r := 0; r < resultChanLength; r++ {
		res = append(res, <-results)
	}

	return res
}
