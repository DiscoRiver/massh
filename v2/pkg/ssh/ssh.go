package ssh

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
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
	Host      string
	Port      string
	Network   string
	SSHConfig *ssh.ClientConfig

	Route []*SingleClientConnection

	// Unexported
	sshSession *ssh.Session
	sshClient  *ssh.Client
}

// NewBastionConnection creates a new BastionConnection by dialing the host through the specified route. Error if session for target host cannot be established.
func NewBastionConnection(host, port, network string, sshConfig *ssh.ClientConfig, route []*SingleClientConnection) (*BastionConnection, error) {
	connection := &BastionConnection{
		Host:      host,
		Port:      port,
		Network:   network,
		SSHConfig: sshConfig,

		Route: route,
	}

	bastionConn, err := connection.dialBastionRoute()
	if err != nil {
		return nil, err
	}

	err = connection.generateClient(bastionConn)
	if err != nil {
		return nil, err
	}

	err = connection.generateSession()
	if err != nil {
		return nil, err
	}

	return connection, nil
}

func (b *BastionConnection) RunJob() {

}

func (b *BastionConnection) generateClient(bastion net.Conn) error {
	ncc, chans, reqs, err := ssh.NewClientConn(bastion, formatHostAndPort(b.Host, b.Port), b.SSHConfig)
	if err != nil {
		return fmt.Errorf("couldn't initiate host connection from bastion client: %s", err)
	}

	b.sshClient = ssh.NewClient(ncc, chans, reqs)

	if b.sshClient == nil {
		return fmt.Errorf("ssh client for final host was nil")
	}

	return nil
}

func (b *BastionConnection) generateSession() (err error) {
	b.sshSession, err = b.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("%s, %s", ErrCreateSessionFailed.Error(), err)
	}

	return nil
}

func (b *BastionConnection) dialBastionRoute() (net.Conn, error) {
	var routeClients []*ssh.Client

	for i := range b.Route {
		if len(b.Route) == 1 {
			client, err := ssh.Dial(b.Route[i].Network, formatHostAndPort(b.Route[i].Host, b.Route[i].Port), b.Route[i].SSHConfig)
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}
			// Dial this host using the previous client. Maintain order of the route.
			conn, err := client.Dial(b.Route[i].Network, formatHostAndPort(b.Route[i].Host, b.Route[i].Port))
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}
			return conn, nil
		}

		if i == 0 {
			firstClient, err := ssh.Dial(b.Route[i].Network, formatHostAndPort(b.Route[i].Host, b.Route[i].Port), b.Route[i].SSHConfig)
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}
			routeClients = append(routeClients, firstClient)
		} else if i <= len(b.Route)-2 {
			// Dial this host using the previous client. Maintain order of the route.
			conn, err := routeClients[i-1].Dial(b.Route[i].Network, formatHostAndPort(b.Route[i].Host, b.Route[i].Port))
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}

			ncc, chans, reqs, err := ssh.NewClientConn(conn, formatHostAndPort(b.Route[i].Host, b.Route[i].Port), b.Route[i].SSHConfig)
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}
			routeClients = append(routeClients, ssh.NewClient(ncc, chans, reqs))
		} else if len(b.Route) > 1 {
			// Dial this host using the previous client. Maintain order of the route.
			conn, err := routeClients[i-1].Dial(b.Route[i].Network, formatHostAndPort(b.Route[i].Host, b.Route[i].Port))
			if err != nil {
				return nil, fmt.Errorf("unable to connect to bastion route host (%s), HOP %d/%d: %s", b.Route[i].Host, i+1, len(b.Route), err)
			}

			return conn, nil
		}
	}

	return nil, nil
}

func formatHostAndPort(host, port string) string {
	return host + ":" + port
}
