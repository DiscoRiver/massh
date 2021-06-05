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
		Command: "echo \"YEAH BOI\"",
	}

	sshc := &ssh.ClientConfig{
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts: []string{"192.168.1.117", "192.168.1.118"},
		SSHConfig: sshc,
		Job: j,
		WorkerPool: 10,

	}

	resChan := make(chan Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	cfg.Stream(resChan)

	for {
		d := <-resChan
		go func() {
			err := readStream(d)
			if err != nil {
				fmt.Println(err)
			}
		}()

	}
}

func readStream(res Result) error {
	if res.Error != nil {
		return fmt.Errorf("%s", res.Error)
	}
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Printf("%s: %s", res.Host, d)
		}
	}
}