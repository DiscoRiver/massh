package massh

/*
The functions here are mostly abstractions to make things easier to adapt to new methods.
 */

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
)

const (
	sshAuthSockEnv = "SSH_AUTH_SOCK"
)

// dial is ssh.Dial
func dial(network, host, port string, config *ssh.ClientConfig) (*ssh.Client, error) {
	c, err := ssh.Dial(network, host+":"+port, config)
	if err != nil {
		return nil, err
	}
	return c, nil
}


func dialViaBastionClient(network string, bastionHost string, remoteHost string, port string, config *ssh.ClientConfig) (*ssh.Client, error) {
	bastionClient, err := ssh.Dial(network, bastionHost+":"+port, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to bastion: %s", err)
	}

	remoteHostConn, err := bastionClient.Dial(network, remoteHost+":"+port)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to remote host: %s", err)
	}

	ncc, chans, reqs, err := ssh.NewClientConn(remoteHostConn, remoteHost+":"+port, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create remote host ssh client through bastion: %s", err)
	}

	clientThroughBastion := ssh.NewClient(ncc, chans, reqs)

	return clientThroughBastion, nil
}

// newClientSession is ssh.Client.NewSession
func newClientSession(client *ssh.Client) (*ssh.Session, error){
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

func generateSSHClientWithPotentialBastion(host string, config *Config) (*ssh.Client, error) {
	if config.BastionHost != "" {
		var sshConf *ssh.ClientConfig
		if config.BastionHostSSHConfig != nil {
			sshConf = config.BastionHostSSHConfig
		} else {
			sshConf = config.SSHConfig
		}

		client, err := dialViaBastionClient("tcp", config.BastionHost, host, sshPort, sshConf)
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	client, err := dial("tcp", host, sshPort, config.SSHConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
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
	sshAgent, err := net.Dial("unix", os.Getenv(sshAuthSockEnv))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), nil
}