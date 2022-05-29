package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests are set up for use in the .github/workflows/go.yml workflow.

var (
	testHosts = map[string]struct{}{"localhost": {}}

	testBastionHost = "localhost"

	testJob = &Job{
		Command: "echo \"Hello, World\"",
	}

	testJob2 = &Job{
		Command: "echo \"Hello, World 2\"",
	}

	testJob3 = &Job{
		Command: "echo \"Hello, World 3\"",
	}

	testJobSlow = &Job{
		Command: "echo \"Hello, World\"; sleep 5",
	}

	testSSHConfig = &ssh.ClientConfig{
		User:            "runner",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(2) * time.Second,
	}

	testConfig = &Config{
		Hosts:      testHosts,
		SSHConfig:  testSSHConfig,
		Job:        testJob,
		WorkerPool: 10,
		Stop:       make(chan struct{}, 1),
	}
)

func TestSshCommandStream(t *testing.T) {
	NumberOfStreamingHostsCompleted = 0

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan *Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := testConfig.Stream(resChan)
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
					t.Logf("Unexpected error in stream test for host %s: %s", result.Host, result.Error)
					t.Fail()

					wg.Done()
				} else {
					readStream(result, &wg, t)
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == len(testConfig.Hosts) {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()

				return
			}
		}
	}
}

func TestSshCommandStreamWithSlowHost(t *testing.T) {
	// Remove current singular job.
	jobBackup := testConfig.Job
	testConfig.Job = testJobSlow

	// Specify our slow timeout (remove value at end of func.)
	testConfig.SlowTimeout = 3

	// Must revert when test concludes.
	defer func() {
		testConfig.Job = jobBackup
	}()

	NumberOfStreamingHostsCompleted = 0

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan *Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := testConfig.Stream(resChan)
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
					t.Logf("Unexpected error in stream test for host %s: %s", result.Host, result.Error)
					t.Fail()

					wg.Done()
				} else {
					readStreamSlow(result, &wg, t)
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == len(testConfig.Hosts) {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()

				return
			}
		}
	}
}

// Test for bugs in lots of lines.
func TestSshCommandStreamBigData(t *testing.T) {
	defer func() { testConfig.Job = testJob }()
	NumberOfStreamingHostsCompleted = 0

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	testConfig.Job = &Job{
		Command: "cat /var/log/auth.log",
	}

	resChan := make(chan *Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := testConfig.Stream(resChan)
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
					t.Logf("Unexpected error in stream test for host %s: %s", result.Host, result.Error)
					t.Fail()

					wg.Done()
				} else {
					readStream(result, &wg, t)
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == len(testConfig.Hosts) {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()

				return
			}
		}
	}
}
func readStream(res *Result, wg *sync.WaitGroup, t *testing.T) {
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Print(string(d))
		case <-res.DoneChannel:
			wg.Done()
		}
	}
}

func readStreamSlow(res *Result, wg *sync.WaitGroup, t *testing.T) {
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Print(string(d))
		case <-res.DoneChannel:
			if res.IsSlow != true {
				t.Logf("Host was not flagged as slow.")
				t.Fail()
			}
			wg.Done()
		}
	}
}

func TestSshBulk(t *testing.T) {
	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := testConfig.Run()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	for i := range res {
		if !strings.Contains(string(res[i].Output), "Hello, World") {
			t.Logf("Expected output from bulk test not received from host %s: \n \t Output: %s \n \t Error: %s\n", res[i].Host, res[i].Output, res[i].Error)
			t.Fail()
		}
	}
}

func TestSshBastion(t *testing.T) {
	// Must remove bastion host once test concludes.
	defer func() { testConfig.BastionHost = "" }()
	// Add bastion host to config
	testConfig.SetBastionHost(testBastionHost)

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Logf("Couldn't set private key auth: %s", err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := testConfig.Run()
	if err != nil {
		t.Logf("Run failed to execute: %s", err)
		t.FailNow()
	}

	for i := range res {
		if res[i].Error != nil {
			t.Logf("Unexpected error in bastion test for host %s: %s", res[i].Host, res[i].Error)
			t.Fail()
		}
		if !strings.Contains(string(res[i].Output), "Hello, World") {
			t.Logf("Expected output from bastion test not received from host %s: \n \t Output: %s \n \t Error: %s\n", res[i].Host, res[i].Output, res[i].Error)
			t.Fail()
		}
	}
}

func TestBulkWithJobStack(t *testing.T) {
	// Remove current singular job.
	jobBackup := testConfig.Job
	testConfig.Job = nil

	// Must remove jobstack when test concludes.
	defer func() {
		testConfig.JobStack = nil
		testConfig.Job = jobBackup
	}()

	// Add our stack
	testConfig.JobStack = &[]Job{*testJob, *testJob2, *testJob3}

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	res, err := testConfig.Run()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	expectedLength := len(*testConfig.JobStack) * len(testConfig.Hosts)
	if len(res) != expectedLength {
		t.Logf("Expected %d results, got %d", expectedLength, len(res))
		t.FailNow()
	}

	for i := range res {
		if !strings.Contains(string(res[i].Output), "Hello, World") {
			t.Logf("Expected output from bulk test not received from host %s: \n \t Output: %s \n \t Error: %s\n", res[i].Host, res[i].Output, res[i].Error)
			t.FailNow()
		}
		fmt.Printf("%s: %s", res[i].Host, res[i].Output)
	}
}

func TestSshCommandStreamWithJobStack(t *testing.T) {
	// Remove current singular job.
	jobBackup := testConfig.Job
	testConfig.Job = nil

	// Must remove jobstack when test concludes.
	defer func() {
		testConfig.JobStack = nil
		testConfig.Job = jobBackup
	}()

	// Add our stack.
	testConfig.JobStack = &[]Job{*testJob, *testJob2, *testJob3}

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan *Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	NumberOfStreamingHostsCompleted = 0
	err := testConfig.Stream(resChan)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var wg sync.WaitGroup
	numberOfExpectedCompletions := len(testConfig.Hosts) * len(*testConfig.JobStack)
	// This can probably be cleaner. We're hindered somewhat, I think, by reading a channel from a channel.
	for {
		select {
		case result := <-resChan:
			wg.Add(1)
			go func() {
				if result.Error != nil {
					t.Logf("Unexpected error in stream test for host %s: %s", result.Host, result.Error)
					t.Fail()

					wg.Done()
				} else {
					readStream(result, &wg, t)
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == numberOfExpectedCompletions {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()
				return
			}
		}
	}
}

func TestSSHCommandStreamStop(t *testing.T) {
	NumberOfStreamingHostsCompleted = 0

	jobBackup := testConfig.Job

	defer func() {
		testConfig.Job = jobBackup
	}()

	// We want a continuous job here
	testConfig.Job = &Job{
		Command: "hexdump -C /dev/urandom > /dev/null",
	}

	if err := testConfig.SetPrivateKeyAuth("~/.ssh/id_rsa", ""); err != nil {
		t.Log(err)
		t.FailNow()
	}

	resChan := make(chan *Result)

	// This should be the last responsibility from the massh package. Handling the Result channel is up to the user.
	err := testConfig.Stream(resChan)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// Close the session after 3 seconds. I think it's fine to just sleep here.
	go func() {
		time.Sleep(3 * time.Second)
		testConfig.StopAllSessions()
	}()

	var wg sync.WaitGroup
	// This can probably be cleaner. We're hindered somewhat, I think, by reading a channel from a channel.
	for {
		select {
		case result := <-resChan:
			wg.Add(1)
			go func() {
				if result.Error != nil {
					t.Logf("Unexpected error in stream test for host %s: %s", result.Host, result.Error)
					t.Fail()

					wg.Done()
				} else {
					readStream(result, &wg, t)
				}
			}()
		default:
			if NumberOfStreamingHostsCompleted == len(testConfig.Hosts) {
				// We want to wait for all goroutines to complete before we declare that the work is finished, as
				// it's possible for us to execute this code before the gofunc above has completed if left unchecked.
				wg.Wait()

				return
			}
		}
	}
}
