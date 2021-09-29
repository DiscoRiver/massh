package main

import (
	"fmt"
	"github.com/discoriver/massh"
	"golang.org/x/crypto/ssh"
	"sync"
	"time"
)

func main() {
	j := massh.Job{
		Command: "echo \"Hello, World\"",
	}

	j2 := massh.Job{
		Command: "echo \"Hello, World 2\"",
	}

	j3 := massh.Job{
		Command: "echo \"Hello, World 3\"",
	}

	sshc := &ssh.ClientConfig{
		// Fake credentials
		User:            "u01",
		Auth:            []ssh.AuthMethod{ssh.Password("password")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &massh.Config{
		// In this example I was testing with two working hosts, and two non-existent IPs.
		SSHConfig:  sshc,
		JobStack:   &[]massh.Job{j, j2, j3},
		WorkerPool: 10,
	}
	cfg.SetHosts([]string{"192.168.1.119", "192.168.1.120", "192.168.1.129", "192.168.1.212"})

	resChan := make(chan massh.Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := cfg.Stream(resChan)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	numberOfExpectedCompletions := len(cfg.Hosts) * len(*cfg.JobStack)
	// This can probably be cleaner. We're hindered somewhat, I think, by reading a channel from a channel.
	for {
		select {
		case result := <-resChan:
			wg.Add(1)
			go func() {
				if result.Error != nil {
					fmt.Printf("%s: %s\n", result.Host, result.Error)
					wg.Done()
				} else {
					err := readStream(result, &wg)
					if err != nil {
						panic(err)
					}
				}
			}()
		default:
			if massh.NumberOfStreamingHostsCompleted == numberOfExpectedCompletions  {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()

				// This should always be the last thing written. Waiting above ensures this.
				fmt.Println("Everything returned.")
				return
			}
		}
	}
}

// Read Stdout stream
func readStream(res massh.Result, wg *sync.WaitGroup) error {
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Printf("%s: %s", res.Host, d)
		case <-res.DoneChannel:
			// Confirm that the host has exited.
			fmt.Printf("%s: Finished\n", res.Host)
			wg.Done()
		}
	}
}
