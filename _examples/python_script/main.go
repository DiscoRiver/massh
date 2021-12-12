package main

import (
	"fmt"
	"github.com/discoriver/massh"
	"golang.org/x/crypto/ssh"
	"time"
)

func main() {
	j := &massh.Job{
		Command: "echo \"Hello, World\"",
	}

	sshc := &ssh.ClientConfig{
		// Fake credentials
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := massh.NewConfig()
	cfg.SSHConfig = sshc
	cfg.Job = j
	cfg.WorkerPool = 10
	cfg.SetHosts([]string{"192.168.1.118", "192.168.1.123"})

	err := cfg.Job.SetScript("script.py", "")
	if err != nil {
		panic(err)
	}

	res, err := cfg.Run()
	if err != nil {
		panic(err)
	}

	for i := range res {
		if res[i].Error != nil {
			fmt.Printf("%s: %s\n", res[i].Host, res[i].Error)
		} else {
			fmt.Printf("%s: %s", res[i].Host, res[i].Output)
		}
	}
}
