package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"testing"
	"time"
)

// Testing for development, should be moved/emoved before merging to main.
//
// Credentials are fine to leave here for ease-of-use, as it's an isolated Linux box.
func TestSshCommandStream(t *testing.T) {
	j := &Job{
		Command: "hexdump -C /dev/urandom",
	}

	sshc := &ssh.ClientConfig{
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(10) * time.Second,
	}

	var stdout = make(chan []byte)
	var stderr = make(chan []byte)

	cfg := &Config{
		Hosts: []string{"192.168.1.114"},
		SSHConfig: sshc,
		Job: j,
		StdoutStream: stdout,
		StderrStream: stderr,
		WorkerPool: 10,
	}

	go readStream(stdout)
	go readStream(stderr)

	err := cfg.Stream()
	if err != nil {
		panic(err)
	}
}

func readStream(stream chan []byte) {
	timer := time.NewTimer(time.Second * 10)
	defer timer.Stop()
	for {
		select {
		case d := <-stream:
			fmt.Printf("%s", d)
			// Check for regexp match in S.Buffer
		case <-timer.C:
			// Handle timeout
			return
		}
	}
}