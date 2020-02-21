package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"log"
	"github.com/discoriver/massh/massh"
	"os"
	"os/user"
	"syscall"
	"time"
)

/*
Right now everything here is designed as a proof of concept. Things in main need to be worked out,
but for now simply proving that the massh package is behaving as expected is enough.

TODO: Look at what needs to be handled by the massh package that is currently in main.
 */
func main() {
	parseCommands()

	mConfig := masshConfigBuilder()

	fmt.Print(mConfig.Run())
}

func readPassword(prompt string) ssh.AuthMethod {
	fmt.Fprint(os.Stderr, prompt)
	var fd int
	if terminal.IsTerminal(syscall.Stdin) {
		fd = syscall.Stdin
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}
	bytePassword, err := terminal.ReadPassword(fd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr)
	return ssh.Password(string(bytePassword))
}

func masshConfigBuilder() *massh.Config {
	config := &massh.Config{
		Hosts: command.Hosts,
		SSHConfig: &ssh.ClientConfig{
			User: command.User,
			Auth: []ssh.AuthMethod{},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout: time.Duration(command.Timeout) * time.Second,
		},
		Job: &massh.Job{},
		WorkerPool: command.WorkerPool,
	}

	var signer ssh.Signer
	if command.PublicKey != "" {
		signer = getSigner(command.PublicKey)
		config.SSHConfig.Auth = append(config.SSHConfig.Auth, ssh.PublicKeys(signer))
	} else {
		config.SSHConfig.Auth = append(config.SSHConfig.Auth, readPassword("Enter SSH Password: "))
	}


	if command.Script != "" {
		err := config.Job.SetLocalScript(command.Script, command.ScriptArgs)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		config.Job.SetCommand(command.Command)
	}
	return config
}

func getSigner(s string) ssh.Signer {
	// read private key file
	key, err := ioutil.ReadFile(s)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}
	return signer
}

func findUserHome() string {
	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Couldn't find user home: %s", err)
		os.Exit(1)
	}
	return usr.HomeDir
}