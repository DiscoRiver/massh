package massh

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
)

type Result struct {
	Host string
	Job string
	Output string
}

func getScript(s *ssh.Session, j *Job) string {
	// Set up remote script
	if j.script != nil {
		s.Stdin = bytes.NewReader(j.script)
		return "cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh"
	} else {
		return j.Command

	}
}
func sshCommand(host string, j *Job, sshConf *ssh.ClientConfig) Result {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, "22"), sshConf)
	if err != nil {
		return Result{host, "", fmt.Sprintf("unable to connect: %v", err)}
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer session.Close()

	job := getScript(session, j)

	// run the job
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(job); err != nil {
		log.Fatal("Failed to run: " + err.Error())
	}
	return Result{host, job, b.String()}
}

func worker(hosts <- chan string, results chan<- Result, job *Job, sshConf *ssh.ClientConfig) {
	for host := range hosts {
		results <- sshCommand(host, job, sshConf)
	}
}

func run(c *Config) {
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	for i := 0; i < c.WorkerPool; i++ {
		go worker(hosts, results, c.Job, c.SSHConfig)
	}

	for j := 0; j < len(c.Hosts); j++ {
		hosts <- c.Hosts[j]
	}
	close(hosts)

	var res []Result
	for r := 0; r < len(c.Hosts); r++ {
		res = append(res, <-results)
	}
	fmt.Print(res)
}