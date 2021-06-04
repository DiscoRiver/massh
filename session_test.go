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
		Command: "echo \"Hello\"; sleep 5; echo \"World\"",
	}

	sshc := &ssh.ClientConfig{
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(10) * time.Second,
	}

	stdout := make(chan []byte)
	stderr := make(chan []byte)

	cfg := &Config{
		Hosts: []string{"192.168.1.116", "192.168.1.116"},
		SSHConfig: sshc,
		Job: j,
		WorkerPool: 10,
		StdoutStream: stdout,
		StderrStream: stderr,
	}

	go readStream(stdout)
	go readStream(stderr)

	cfg.Stream()
}

func readStream(stream chan []byte) {
	duration := time.Second * 20
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case d := <-stream:
			fmt.Printf("%s", d)
			timer.Reset(duration)
		case <-timer.C:
			// Handle timeout
			close(stream)
			fmt.Println("chan closed")
		}
	}
}