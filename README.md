![logo](./doc/logo.jpg)

[![Go Report Card](https://goreportcard.com/badge/github.com/DiscoRiver/massh)](https://goreportcard.com/report/github.com/DiscoRiver/massh) ![Go Report Card](https://img.shields.io/github/license/DiscoRiver/massh) [![Go Doc](https://img.shields.io/badge/GoDoc-Available-informational)](https://godoc.org/github.com/DiscoRiver/massh/massh)

### Description
Go package for running Linux distributed shell commands via SSH. 

### Why?
I wanted to experiment with distributed SSH commands, and provide a functional replacement for the old, 
stale [omnissh](https://github.com/rykugur/omnissh) repository.

### Example:

```
package main

import "github.com/discoriver/massh/massh"

func main() {
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
```

### Usage:
Get the massh package;

```go get github.com/DiscoRiver/massh/massh```

### Documentation

* [GoDoc](https://godoc.org/github.com/DiscoRiver/massh/massh)

### Notes
Right now you can either user this repo as-is, which provides simple usage, or you can import the massh
package and use your own behaviour for building and running the Config.

Output is limited to printing a slice of Results. Additional output processing is required for anything
fancy. Result is a struct containing the host, the command, and the output (which includes the newline). 
This should be enough information for all your basic output needs. No status codes are returned. 

When specifying a script, it's contents will be added to stdin, and then the following command will be
executed to run it on the remote machine;

```cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh && rm ./outfile.sh```

