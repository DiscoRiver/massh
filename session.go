package massh

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
)

// Result contains usable output from SSH commands.
type Result struct {
	Host   string // Hostname
	Job    string // The command that was run
	Output []byte
	Error  error
	StdOutStream chan []byte
	StdErrStream chan []byte
}

// getJob determines the type of job and returns the command string
func getJob(s *ssh.Session, j *Job) string {
	// Set up remote script
	if j.script != nil {
		s.Stdin = bytes.NewReader(j.script)
		return "cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh && rm ./outfile.sh"
	}

	return j.Command
}

// sshCommand creates ssh.Session and runs the specified job.
func sshCommand(host string, j *Job, sshConf *ssh.ClientConfig) Result {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, "22"), sshConf)
	if err != nil {
		return Result{host, "", nil, fmt.Errorf("unable to connect: %v", err), nil, nil}
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer session.Close()

	job := getJob(session, j)

	// run the job
	var b bytes.Buffer
	var r Result
	session.Stdout = &b
	if err := session.Run(job); err != nil {
		r.Error = err
	}
	r.Host = host
	r.Job = job
	r.Output = b.Bytes()
	
	return r
}


// TODO: Find a way to associate output in channel to a specific host. Currently, everything will be streamed to a single channel which is not ideal for processing purposes.
func sshCommandStream(host string, j *Job, sshConf *ssh.ClientConfig, resChan chan Result) (err error) {
	r := Result{}
	r.Host = host

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, "22"), sshConf)
	if err != nil {
		r.Error = fmt.Errorf("unable to connect: %v", err)
		resChan <- r
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		r.Error = fmt.Errorf("failed to create session: %s", err)
		resChan <- r
		return
	}
	defer session.Close()

	job := getJob(session, j)
	r.Job = job

	var StdOutPipe io.Reader
	StdOutPipe, err = session.StdoutPipe()
	if err != nil {
		r.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
	}

	var StdErrPipe io.Reader
	StdErrPipe, err = session.StderrPipe()
	if err != nil {
		r.Error = fmt.Errorf("could not set StdOutPipe: %s", err)
	}

	stdout := make(chan []byte)
	stderr := make(chan []byte)
	r.StdOutStream = stdout
	r.StdErrStream = stderr

	// Using a goroutine here so we can reade from StdOutPipe as it's populated, rather than
	// only once the command has finished executing.
	go reader(StdOutPipe, r.StdOutStream)
	go reader(StdErrPipe, r.StdErrStream)

	if err := session.Start(job); err != nil {
		r.Error = fmt.Errorf("could not start job: %s", err)
	}

	resChan <- r

	// Need to use session.Start, and session.Wait after we initiate the gorountine to Reader,
	// rather than using session.Run, which waits for the command before writing the output.
	//
	// This is important for us to be able to stream the output to the stream channel.
	session.Wait()

	return nil
}

func reader(rdr io.Reader, stream chan []byte) {
	var data = make([]byte, 1024)
	for {
		n, err := rdr.Read(data)
		if err != nil {
			// Handle error
			return
		}
		stream <- data[:n]
	}
}

// worker invokes sshCommand for each host in the channel
func worker(hosts <-chan string, results chan<- Result, job *Job, sshConf *ssh.ClientConfig, resChan chan Result) {
	if resChan == nil {
		for host := range hosts {
			results <- sshCommand(host, job, sshConf)
		}
	} else {
		for host := range hosts {
			fmt.Println("Streaming.")
			if err := sshCommandStream(host, job, sshConf, resChan); err != nil {
				// write error
			}
			//TODO: This is all wrong
			results <- Result{}
		}
	}
}

// run sets up goroutines, worker pool, and returns the command results.
func runStream(c *Config, rs chan Result) {
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c.Job, c.SSHConfig, rs)
	}

	for j := 0; j < len(c.Hosts); j++ {
		hosts <- c.Hosts[j] // send each host to the channel
	}
	close(hosts)
}

// run sets up goroutines, worker pool, and returns the command results.
func run(c *Config) (res []Result) {
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c.Job, c.SSHConfig, nil)
	}

	for j := 0; j < len(c.Hosts); j++ {
		hosts <- c.Hosts[j] // send each host to the channel
	}
	close(hosts)

	for r := 0; r < len(c.Hosts); r++ {
		res = append(res, <-results)
	}

	return res
}