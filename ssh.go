package massh

/*
The functions here are mostly abstractions to make things easier to adapt to new methods.
 */

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
)

// dial is ssh.Dial
func dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	c, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// newClientSession is ssh.Client.NewSession
func newClientSession(client *ssh.Client) (*ssh.Session, error){
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

// runJob is ssh.Session.Run
func runJob(session *ssh.Session, job string) error {
	if err := session.Run(job); err != nil {
		return err
	}
	return nil
}

// startJob is ssh.Session.Start
func startJob(session *ssh.Session, job string) error {
	if err := session.Start(job); err != nil {
		return err
	}
	return nil
}

func sshAuthSock() (ssh.AuthMethod, error) {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), nil
}