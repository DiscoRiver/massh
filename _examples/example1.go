package _examples

import "github.com/discoriver/massh/massh"

func example1() {
	// Create pointers to config & job
	config := &massh.Config{}
	job := &massh.Job{
		Command: "echo hello world",
	}

	config.SetHosts([]string{"host1", "host2"})

	err := config.SetPublicKeyAuth("~/.ssh/id_rsa")
	if err != nil {
		panic(err)
	}

	config.SetJob(job)
	config.SetWorkerPool(2)

	config.Run()
}
