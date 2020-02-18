package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
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

	myconfig := &Config{
		Hosts: []string{"host1", "host2"},
		SSHConfig: config,
		Job: &Job{
			Commands: []string{"/usr/bin/whoami"},
		},
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