package massh

import (
	"golang.org/x/crypto/ssh"
	"testing"
)

func TestSuccesscheckConfigSanity(t *testing.T) {

	// This config should be valid
	goodConfig := &Config{
		Hosts: []string{"host1", "host2"},
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
	expectedErrorString := "sanity check failed, the following config values are not set: [Hosts Job SSHConfig WorkerPool]"
	if err.Error() != expectedErrorString {
		t.Errorf("Error did not match expected string.\nGot: %s\nExpected: %s\n", err.Error(), expectedErrorString)
	}
}