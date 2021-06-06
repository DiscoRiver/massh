package main

import (
	"fmt"
	"github.com/discoriver/massh"
	"golang.org/x/crypto/ssh"
	"os"
	"time"
)

/*
right now everything here is designed as a proof of concept. Things in main need to be worked out,
but for now simply proving that the massh package is behaving as expected is enough.
*/
func main() {
	parseCommands()

	//mConfig := masshConfigBuilder()
	mConfig := massh.Config{}

	if err := mConfig.CheckSanity(); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(0)
	}
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
