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

	defaultBastionHop = &SingleClientConnection{
		Host:      "localhost",
		Port:      "22",
		Network:   "tcp",
		SSHConfig: defaultSSHClientConfig,
	}

	brokenBastionHop = &SingleClientConnection{
		Host:      "localhost",
		Port:      "22",
		Network:   "tcp",
		SSHConfig: brokenSSHClientConfig,
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

func TestNewBastionConnection_Success(t *testing.T) {
	var bastionRoute = []*SingleClientConnection{defaultBastionHop}

	_, err := NewBastionConnection("localhost", "22", "tcp", defaultSSHClientConfig, bastionRoute)
	if err != nil {
		t.Logf("Failed to dial bastion route: %s", err)
		t.FailNow()
	}
}

func TestNewBastionConnection_Failure_Bastion(t *testing.T) {
	var bastionRoute = []*SingleClientConnection{brokenBastionHop}

	_, err := NewBastionConnection("localhost", "22", "tcp", defaultSSHClientConfig, bastionRoute)
	if err == nil {
		t.Log("Expected error, but received nil.")
		t.FailNow()
	}
}

func TestNewBastionConnection_Failure_Target(t *testing.T) {
	var bastionRoute = []*SingleClientConnection{defaultBastionHop}

	_, err := NewBastionConnection("localhost", "22", "tcp", brokenSSHClientConfig, bastionRoute)
	if err == nil {
		t.Log("Expected error, but received nil.")
		t.FailNow()
	}
}

func TestSingleClientConnectionReconnect_Active(t *testing.T) {
	conn, err := NewSingleClientConnection("localhost", "22", "tcp", defaultSSHClientConfig)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer conn.sshClient.Close() // may error, but we don't really care.

	err = conn.Reconnect()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}

func TestSingleClientConnectionReconnect_Nil(t *testing.T) {
	conn, err := NewSingleClientConnection("localhost", "22", "tcp", defaultSSHClientConfig)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer conn.sshClient.Close()

	// Close connection
	conn.sshClient.Close()

	err = conn.Reconnect()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}

func TestBastionClientConnectionReconnect_Active(t *testing.T) {
	var bastionRoute = []*SingleClientConnection{defaultBastionHop}

	conn, err := NewBastionConnection("localhost", "22", "tcp", defaultSSHClientConfig, bastionRoute)
	if err != nil {
		t.Logf("Failed to dial bastion route: %s", err)
		t.FailNow()
	}
	defer conn.sshClient.Close()

	err = conn.Reconnect()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}

func TestBastionClientConnectionReconnect_Nil(t *testing.T) {
	var bastionRoute = []*SingleClientConnection{defaultBastionHop}

	conn, err := NewBastionConnection("localhost", "22", "tcp", defaultSSHClientConfig, bastionRoute)
	if err != nil {
		t.Logf("Failed to dial bastion route: %s", err)
		t.FailNow()
	}
	defer conn.sshClient.Close()

	// Close connection
	conn.sshClient.Close()

	err = conn.Reconnect()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}
