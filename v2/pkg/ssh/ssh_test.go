package ssh_test

import (
	massh "github.com/discoriver/massh/v2/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	defaultBastionHop = &massh.SingleClientConnection{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
	}

	brokenBastionHop = &massh.SingleClientConnection{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: brokenSSHClientConfig,
	}
)

func TestNewSingleClientConnection_Success(t *testing.T) {
	essentials := massh.NewSingleClientConnectionEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
	}

	conn, err := massh.NewSingleClientConnection(essentials)
	require.NoError(t, err)

	defer conn.Close() // may error, but we don't really care.

	assert.NotNil(t, conn)
}

func TestNewSingleClientConnection_Failure(t *testing.T) {
	essentials := massh.NewSingleClientConnectionEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: brokenSSHClientConfig,
	}

	conn, err := massh.NewSingleClientConnection(essentials)
	require.Error(t, err)

	assert.Nil(t, conn)
}

func TestNewBastionConnection_Success(t *testing.T) {
	var bastionRoute = []*massh.SingleClientConnection{defaultBastionHop}

	essentials := massh.NewBastionClientEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
		Route:     bastionRoute,
	}

	_, err := massh.NewBastionConnection(essentials)
	assert.NoError(t, err)
}

func TestNewBastionConnection_Failure_Bastion(t *testing.T) {
	var bastionRoute = []*massh.SingleClientConnection{brokenBastionHop}

	essentials := massh.NewBastionClientEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
		Route:     bastionRoute,
	}

	_, err := massh.NewBastionConnection(essentials)
	assert.Error(t, err)
}

func TestNewBastionConnection_Failure_Target(t *testing.T) {
	var bastionRoute = []*massh.SingleClientConnection{defaultBastionHop}

	essentials := massh.NewBastionClientEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: brokenSSHClientConfig,
		Route:     bastionRoute,
	}

	_, err := massh.NewBastionConnection(essentials)
	assert.Error(t, err)
}

func TestSingleClientConnectionReconnect_Active(t *testing.T) {
	essentials := massh.NewSingleClientConnectionEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
	}

	conn, err := massh.NewSingleClientConnection(essentials)
	require.NoError(t, err)

	defer conn.Close() // may error, but we don't really care.

	err = conn.Reconnect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestSingleClientConnectionReconnect_Nil(t *testing.T) {
	essentials := massh.NewSingleClientConnectionEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
	}

	conn, err := massh.NewSingleClientConnection(essentials)
	require.NotNil(t, conn)
	require.NoError(t, err)

	defer conn.Close()

	// Close connection
	err = conn.Close()
	require.NoError(t, err)

	err = conn.Reconnect()
	assert.NoError(t, err)
}

func TestBastionClientConnectionReconnect_Active(t *testing.T) {
	var bastionRoute = []*massh.SingleClientConnection{defaultBastionHop}

	essentials := massh.NewBastionClientEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
		Route:     bastionRoute,
	}

	conn, err := massh.NewBastionConnection(essentials)
	assert.NoError(t, err)

	defer conn.Close()

	err = conn.Reconnect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestBastionClientConnectionReconnect_Nil(t *testing.T) {
	var bastionRoute = []*massh.SingleClientConnection{defaultBastionHop}

	essentials := massh.NewBastionClientEssentials{
		Host:      "localhost",
		Port:      "22",
		Network:   massh.TCP,
		SSHConfig: defaultSSHClientConfig,
		Route:     bastionRoute,
	}

	conn, err := massh.NewBastionConnection(essentials)
	require.NoError(t, err)

	defer conn.Close()

	// Close connection
	err = conn.Close()
	require.NoError(t, err)

	err = conn.Reconnect()
	assert.NoError(t, err)
}
