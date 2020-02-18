package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"massh/massh"
	"os"
	"os/user"
	"time"
)

func main() {
	key, err := ioutil.ReadFile(fmt.Sprintf("%s/.ssh/linux_rsa", findUserHome()))
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: "u01",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 10 * time.Second,
	}

	myconfig := &massh.Config{
		Hosts: []string{"172.16.226.25", "172.16.226.26"},
		SSHConfig: config,
		Job: &massh.Job{},
		WorkerPool: 2,
	}

	err = myconfig.Job.SetLocalScript("test.sh", "")
	if err != nil {
		fmt.Println(err)
	}
	myconfig.Run()
}

func findUserHome() string {
	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Couldn't find user home: %s", err)
		os.Exit(1)
	}
	return usr.HomeDir
}