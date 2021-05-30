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
		return Result{host, "", nil, fmt.Errorf("unable to connect: %v", err)}
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
func sshCommandStream(host string, j *Job, sshConf *ssh.ClientConfig, stdoutStream chan []byte, stderrStream chan []byte) (err error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, "22"), sshConf)
	if err != nil {
		return fmt.Errorf("unable to connect: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer session.Close()

	job := getJob(session, j)

	var StdOutPipe io.Reader
	StdOutPipe, err = session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not set StdOutPipe: %s", err)
	}

	var StdErrPipe io.Reader
	StdErrPipe, err = session.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not set StdOutPipe: %s", err)
	}

	if err := session.Start(job); err != nil {
		panic(err)
	}

	// Using a goroutine here so we can reade from StdOutPipe as it's populated, rather than
	// only once the command has finished executing.
	go reader(StdOutPipe, stdoutStream)
	go reader(StdErrPipe, stderrStream)

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
func worker(hosts <-chan string, results chan<- Result, job *Job, sshConf *ssh.ClientConfig, stdout chan []byte, stderr chan []byte) {
	if stdout == nil || stderr == nil {
		for host := range hosts {
			results <- sshCommand(host, job, sshConf)
		}
	} else {
		for host := range hosts {
			if err := sshCommandStream(host, job, sshConf, stdout, stderr); err != nil {
				// write error
			}
		}
	}
}

// run sets up goroutines, worker pool, and returns the command results.
func run(c *Config) (res []Result) {
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	// Set up a worker pool that will accept hosts on the hosts channel.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c.Job, c.SSHConfig, c.StdoutStream, c.StderrStream)
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
