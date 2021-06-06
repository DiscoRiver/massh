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
		Command: "echo \"Hello, World\"",
	}

	sshc := &ssh.ClientConfig{
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      []string{"192.168.1.119", "192.168.1.120", "192.168.1.129", "192.168.1.212"},
		SSHConfig:  sshc,
		Job:        j,
		WorkerPool: 10,
	}

	resChan := make(chan Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	cfg.Stream(resChan)

	for {
		select {
		case d := <-resChan:
			go func() {
				err := readStream(d)
				if err != nil {
					fmt.Println(err)
				}
			}()
		default:
			if Returned == len(cfg.Hosts) {
				return
			}
		}
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