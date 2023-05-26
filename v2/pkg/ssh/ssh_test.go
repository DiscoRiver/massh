package ssh

import (
	"golang.org/x/crypto/ssh"
	"testing"
	"time"
)

var (
	defaultSSHClientConfig = &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password("test")},
		Timeout:         time.Duration(2) * time.Second, // to keep things snappy
	}

	brokenSSHClientConfig = &ssh.ClientConfig{
		User:            "broken",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password("broken")},
		Timeout:         time.Duration(2) * time.Second, // to keep things snappy
	}
)

func TestNewSingleClientConnection_Success(t *testing.T) {
	conn, err := NewSingleClientConnection("localhost", "22", "tcp", defaultSSHClientConfig)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer conn.sshClient.Close() // may error, but we don't really care.
}

func TestNewSingleClientConnection_Failure(t *testing.T) {
	conn, err := NewSingleClientConnection("localhost", "22", "tcp", brokenSSHClientConfig)
	if err == nil {
		t.Log("Expected error, but received nil.")
		t.FailNow()
	}
	defer func() {
		if conn != nil {
			conn.sshClient.Close()
		}
	}()
}
