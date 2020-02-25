package massh

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
)

// Result contains usable output from SSH commands.
type Result struct {
	Host   string
	Job    string
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
	session.Stdout = &b
	if err := session.Run(job); err != nil {
		log.Fatal("Failed to run: " + err.Error())
	}

	return Result{host, job, b.Bytes(), nil}
}

// worker invokes sshCommand for each host in the channel
func worker(hosts <-chan string, results chan<- Result, job *Job, sshConf *ssh.ClientConfig) {
	for host := range hosts {
		results <- sshCommand(host, job, sshConf)
	}
}

// run sets up goroutines, worker pool, and returns the command results.
func run(c *Config) (res []Result) {
	// Channels length is always how many hosts we have
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	// Set up a basic worker pool.
	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c.Job, c.SSHConfig)
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
