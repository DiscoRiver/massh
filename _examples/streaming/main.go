package main

import (
	"fmt"
	"github.com/discoriver/massh"
	"golang.org/x/crypto/ssh"
	"sync"
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
	cfg.SetHosts([]string{"192.168.1.118", "192.168.1.119", "192.168.1.120", "192.168.1.129", "192.168.1.212"})

	resChan := make(chan massh.Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := cfg.Stream(resChan)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	// This can probably be cleaner. We're hindered somewhat, I think, by reading a channel from a channel.
	for {
		select {
		case result := <-resChan:
			wg.Add(1)
			go func() {
				// Need to handle any errors as the existence of this value indicates that the ssh task wasn't started
				// due to some functional error.
				//
				// The reason for this design is that it was important to me not to have the cfg.Stream function return
				// anything, and having it as part of the Result means we can more easily associate the error with a
				// host.
				if result.Error != nil {
					fmt.Printf("%s: %s\n", result.Host, result.Error)
					wg.Done()
				} else {
					readStream(result, &wg)
				}
			}()
		default:
			if massh.NumberOfStreamingHostsCompleted == len(cfg.Hosts) {
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

// An example of how you might add a timeout for inactivity if a command is likely to hang forever.
func readStreamWithTimeout(res massh.Result, wg *sync.WaitGroup, t time.Duration) {
	timeout := time.Second * t
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Printf("%s: %s", res.Host, d)
			timer.Reset(timeout)
		case <-res.DoneChannel:
			// Confirm that the host has exited.
			fmt.Printf("%s: Finished\n", res.Host)
			timer.Reset(timeout)
			wg.Done()
		case <-timer.C:
			fmt.Printf("%s: Timeout due to inactivity\n", res.Host)
			wg.Done()
		}
	}
}
