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

	if err := checkConfigSanity(badConfig); err == nil {
		t.Errorf("Expected failure, got success.")
	}
}