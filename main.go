package main

import (
	"fmt"
	"github.com/discoriver/massh/massh"
	"golang.org/x/crypto/ssh"
	"os"
	"os/user"
	"time"
)

/*
right now everything here is designed as a proof of concept. Things in main need to be worked out,
but for now simply proving that the massh package is behaving as expected is enough.
*/
func main() {
	parseCommands()

	mConfig := masshConfigBuilder()

	fmt.Print(mConfig.Run())
}

func masshConfigBuilder() *massh.Config {
	config := &massh.Config{
		Hosts: command.Hosts,
		SSHConfig: &ssh.ClientConfig{
			User:            command.User,
			Auth:            []ssh.AuthMethod{},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Duration(command.Timeout) * time.Second,
		},
		Job:        &massh.Job{},
		WorkerPool: command.WorkerPool,
	}

	if command.PublicKey != "" {
		if err := config.SetPublicKeyAuth(command.PublicKey); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		pass, err := massh.ReadPassword("Enter SSH Password: ")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		config.SetPasswordAuth(pass)
	}

	if command.Script != "" {
		err := config.Job.SetLocalScript(command.Script, command.ScriptArgs)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		config.Job.SetCommand(command.Command)
	}
	return config
}

func findUserHome() string {
	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Couldn't find user home: %s", err)
		os.Exit(1)
	}
	return usr.HomeDir
}
