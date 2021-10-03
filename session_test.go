package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"sync"
	"testing"
	"time"
)

// Credentials are fine to leave here for ease-of-use, as it's an isolated Linux box.
//
// I'm leaving this test (which is being use in examples), here so I can re-use it in the future.

type sshTestParameters struct {
	Hosts map[string]struct{}
	User string
	Password string
}

func TestSshCommandStream(t *testing.T) {
	NumberOfStreamingHostsCompleted = 0

	testParams := sshTestParameters{
		Hosts: map[string]struct{}{
			"localhost": struct{}{},
		},

	}

	j := &Job{
		Command: "echo \"Hello, World\"",
	}

	sshc := &ssh.ClientConfig{
		User: "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      testParams.Hosts,
		SSHConfig:  sshc,
		Job:        j,
		WorkerPool: 10,
	}

	if err := cfg.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := cfg.Stream(resChan)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var wg sync.WaitGroup
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
						t.Log(err)
						t.FailNow()
					}
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == len(cfg.Hosts) {
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

func readStream(res Result, wg *sync.WaitGroup) error {
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Printf("%s: %s", res.Host, d)
		case <-res.DoneChannel:
			fmt.Printf("%s: Finished\n", res.Host)
			wg.Done()
		}
	}
}

func TestSshBulk(t *testing.T) {
	testParams := sshTestParameters{
		Hosts: map[string]struct{}{
			"localhost": struct{}{},
		},
	}

	j := &Job{
		Command: "echo \"Hello, World\"",
	}

	sshc := &ssh.ClientConfig{
		User: "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      testParams.Hosts,
		SSHConfig:  sshc,
		Job:        j,
		WorkerPool: 10,
	}

	if err := cfg.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := cfg.Run()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	for i := range res {
		fmt.Printf("%s:: OUTPUT: %s ERROR: %s\n", res[i].Host, res[i].Output, res[i].Error)
	}
}

func TestSshBastion(t *testing.T) {
	testParams := sshTestParameters{
		Hosts: map[string]struct{}{
			"localhost": struct{}{},
		},
	}

	j := &Job{
		Command: "echo \"Hello, World\"",
	}

	sshc := &ssh.ClientConfig{
		User: "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      testParams.Hosts,
		SSHConfig:  sshc,
		Job:        j,
		WorkerPool: 10,
		BastionHost: "localhost",
	}

	if err := cfg.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := cfg.Run()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	for i := range res {
		if res[i].Error != nil {
			fmt.Printf("%s:: OUTPUT: %s ERROR: %s\n", res[i].Host, res[i].Output, res[i].Error)
			t.FailNow()
		}
		fmt.Printf("%s:: OUTPUT: %s ERROR: %s\n", res[i].Host, res[i].Output, res[i].Error)
	}
}

func TestBulkWithJobStack(t *testing.T) {
	testParams := sshTestParameters{
		Hosts: map[string]struct{}{
			"localhost": struct{}{},
		},
	}

	j := Job{
		Command: "echo \"Hello, World\"",
	}

	j2 := Job{
		Command: "echo \"Hello, World 2\"",
	}

	sshc := &ssh.ClientConfig{
		User: "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      testParams.Hosts,
		SSHConfig:  sshc,
		JobStack:   &[]Job{j, j2},
		WorkerPool: 10,
	}

	if err := cfg.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := cfg.Run()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	for i := range res {
		fmt.Printf("%s:: OUTPUT: %s ERROR: %s\n", res[i].Host, res[i].Output, res[i].Error)
	}
}

func TestSshCommandStreamWithJobStack(t *testing.T) {
	testParams := sshTestParameters{
		Hosts: map[string]struct{}{
			"localhost": struct{}{},
		},
	}

	j := Job{
		Command: "echo \"Hello, World\"",
	}

	j2 := Job{
		Command: "echo \"Hello, World 2\"",
	}

	j3 := Job{
		Command: "echo \"Hello, World 3\"",
	}

	sshc := &ssh.ClientConfig{
		User: "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	cfg := &Config{
		Hosts:      testParams.Hosts,
		SSHConfig:  sshc,
		JobStack:   &[]Job{j, j2, j3},
		WorkerPool: 10,
	}

	if err := cfg.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	NumberOfStreamingHostsCompleted = 0
	err := cfg.Stream(resChan)
	if err != nil {
		t.Log(err)
		t.FailNow()
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
						t.Log(err)
						t.FailNow()
					}
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == numberOfExpectedCompletions  {
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