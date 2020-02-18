package main

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

func sshCommand(host string, job string, sshConf *ssh.ClientConfig) Result {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", host, "22"), sshConf)
	if err != nil {
		return Result{host, job, fmt.Sprintf("unable to connect: %v", err)}
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Fatal("Failed to create session: ", err)
	}
	defer session.Close()

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(job); err != nil {
		log.Fatal("Failed to run: " + err.Error())
	}
	return Result{host, job, b.String()}
}

func worker(hosts <- chan string, results chan<- Result, job string, sshConf *ssh.ClientConfig) {
	for host := range hosts {
		results <- sshCommand(host, job, sshConf)
	}
}

func run(c *Config) {
	hosts := make(chan string, len(c.Hosts))
	results := make(chan Result, len(c.Hosts))

	for i := 0; i < 2; i++ {
		go worker(hosts, results, c.Job.Commands[0], c.SSHConfig)
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