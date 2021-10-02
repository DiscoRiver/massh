package massh

import (
	"golang.org/x/crypto/ssh"
	"testing"
)

func TestSuccesscheckConfigSanity(t *testing.T) {

	// This config should be valid
	goodConfig := &Config{
		Hosts: map[string]struct{}{
			"host1": struct{}{},
			"host2": struct{}{},
		},
		SSHConfig: &ssh.ClientConfig{
			User:            "testUser",
			Auth:            []ssh.AuthMethod{
				ssh.Password("password"),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10,
		},
		Job:        &Job{
			Command: "hostname",
		},
		WorkerPool: 10,
	}

	// Check valid config
	if err := checkConfigSanity(goodConfig); err != nil {
		t.Errorf("Expectd nil error, got: %s", err)
	}
}

func TestFailcheckConfigSanity(t *testing.T) {

	// This config should be invalid
	badConfig := &Config{}

	var err error
	if err = checkConfigSanity(badConfig); err == nil {
		t.Error("Expected failure, got success.")
		t.FailNow()
	}

	// Testing this to ensure all unset parameters are returned.
	expectedErrorString := "bad config, the following config items are not correct: [Hosts Jobs SSHConfig WorkerPool]"
	if err.Error() != expectedErrorString {
		t.Errorf("Error did not match expected string.\nGot: %s\nExpected: %s\n", err.Error(), expectedErrorString)
	}
}

func TestCheckJobsWithJob(t *testing.T) {
	j := Job{}

	// No need to enter any other config values for this test.
	badConfig := &Config{
		Job: &j,
		JobStack: &[]Job{j, j},
	}

	var err error
	if err = checkJobs(badConfig); err == nil {
		t.Error("Expected failure, got success.")
		t.FailNow()
	}

	// Testing this to ensure all unset parameters are returned.
	expectedErrorString := ErrJobConflict
	if err.Error() != expectedErrorString.Error() {
		t.Errorf("Error did not match expected string.\nGot: %s\nExpected: %s\n", err.Error(), expectedErrorString)
	}
}

func TestCheckJobsWithoutJob(t *testing.T) {
	// This config should be invalid
	badConfig := &Config{}

	var err error
	if err = checkJobs(badConfig); err == nil {
		t.Error("Expected failure, got success.")
		t.FailNow()
	}

	// Testing this to ensure all unset parameters are returned.
	expectedErrorString := ErrNoJobsSet
	if err.Error() != expectedErrorString.Error() {
		t.Errorf("Error did not match expected string.\nGot: %s\nExpected: %s\n", err.Error(), expectedErrorString)
	}
}