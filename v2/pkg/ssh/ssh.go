package ssh

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
)

const (
	authSockEnv = "SSH_AUTH_SOCK"
)

var (
	ErrNilSession             = errors.New("session is nil")
	ErrCreateSessionFailed    = errors.New("could not create new session")
	ErrClientConnectionFailed = errors.New("could not establish client connection")
)

type SSHConnection interface {
	RunJob(job string) error
	StartJob(job string) error

	Reconnect() error

	GetClient() *ssh.Client
	GetSession() *ssh.Session
}

type SingleClientConnection struct {
	Host      string
	Port      string
	Network   string
	SSHConfig *ssh.ClientConfig

	// Unexported
	sshSession *ssh.Session
	sshClient  *ssh.Client
}

// NewSingleClientConnection creates a new single client connection, and errors if connection cannot be established.
func NewSingleClientConnection(host, port, network string, sshConfig *ssh.ClientConfig) (*SingleClientConnection, error) {
	connection := &SingleClientConnection{
		Host:      host,
		Port:      port,
		Network:   network,
		SSHConfig: sshConfig,
	}

	err := connection.generateClient()
	if err != nil {
		return nil, ErrClientConnectionFailed
	}

	err = connection.generateSession()
	if err != nil {
		return nil, ErrCreateSessionFailed
	}

	return connection, nil
}

func (c *SingleClientConnection) RunJob(job string) error {
	if err := c.sshSession.Run(job); err != nil {
		return err
	}

	return nil
}

func (c *SingleClientConnection) StartJob(job string) error {
	if err := c.sshSession.Start(job); err != nil {
		return err
	}

	return nil
}

// Reconnect reestablishes the SSH client and session.
func (c *SingleClientConnection) Reconnect() error {
	c.sshClient = nil
	c.sshSession = nil

	err := c.generateClient()
	if err != nil {
		return ErrClientConnectionFailed
	}

	err = c.generateSession()
	if err != nil {
		return ErrCreateSessionFailed
	}

	return nil
}

// GetClient exposes the ssh.Client to the caller.
func (c *SingleClientConnection) GetClient() *ssh.Client {
	return c.sshClient
}

// GetSession exposes the ssh.Session to the caller.
func (c *SingleClientConnection) GetSession() *ssh.Session {
	return c.sshSession
}

// generateClient creates an ssh.Client from struct params
func (c *SingleClientConnection) generateClient() (err error) {
	c.sshClient, err = ssh.Dial(c.Network, c.Host+":"+c.Port, c.SSHConfig)
	if err != nil {
		return fmt.Errorf("%s, %s", ErrClientConnectionFailed, err)
	}

	return nil
}

// generateSession creates a session from the struct client
func (c *SingleClientConnection) generateSession() (err error) {
	c.sshSession, err = c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("%s, %s", ErrCreateSessionFailed.Error(), err)
	}

	return nil
}

type BastionConnection struct {
	Route      []*SingleClientConnection
	sshSession *ssh.Session
}

func reconnect[conn SSHConnection]() {

}
