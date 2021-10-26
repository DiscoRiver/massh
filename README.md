![logo](./doc/logo.jpg)

[![Go Test](https://github.com/DiscoRiver/massh/actions/workflows/go-test.yml/badge.svg)](https://github.com/DiscoRiver/massh/actions/workflows/go-test.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/DiscoRiver/massh)](https://goreportcard.com/report/github.com/DiscoRiver/massh)  ![Go Report Card](https://img.shields.io/github/license/DiscoRiver/massh) [![Go Doc](https://img.shields.io/badge/GoDoc-Available-informational)](https://godoc.org/github.com/DiscoRiver/massh)

## Description
Go package for streaming Linux distributed shell commands via SSH. 

What makes Massh special is it's ability to stream & process output concurrently. See `_examples/example_streaming` for some sample code.

## Contribute

Have a question, idea, or something you think can be improved? Open an issue or PR and let's discuss it!

## Example:

```go
package main

import (
	"fmt"
	"github.com/discoriver/massh"
	"golang.org/x/crypto/ssh"
)

func main() {
	// Create pointers to config & job
	config := massh.NewConfig()

	job := &massh.Job{
		Command: "echo hello world",
	}

	config.SetHosts([]string{"192.168.1.130", "192.168.1.125"})

	// Password auth
	config.SetPasswordAuth("u01", "password")

	// Key auth in same config. Auth will try all methods provided before failing.
	err := config.SetPrivateKeyAuth("~/.ssh/id_rsa", "")
	if err != nil {
		panic(err)
	}

	config.SetJob(job)
	config.SetWorkerPool(2)
	config.SetSSHHostKeyCallback(ssh.InsecureIgnoreHostKey())

	// Make sure config will run
	config.CheckSanity()

	res, err := config.Run()
	if err != nil {
		panic(err)
	}

	for i := range res {
		fmt.Printf("%s:\n \t OUT: %s \t ERR: %s\n", res[i].Host, res[i].Output, res[i].Error)
	}
}
```

More examples available in the examples directory.

## Usage:
Get the massh package;

```go get github.com/DiscoRiver/massh```

## Documentation

* [GoDoc](https://godoc.org/github.com/DiscoRiver/massh)

## Other

### Bastion Host

Specify a bastion host and config with `BastionHost` and `BastionHostSSHConfig` in your
`massh.Config`. You may leave `BastionHostSSHConfig` as `nil`, in which case `SSHConfig` will be used instead. The process is
automatic, and if `BastionHost` is not `nil`, it will be used. 

### Streaming output

There is an example of streaming output in the direcotry `_examples/example_streaming`, which contains one method of reading
from the results channel, and processing the output.

Running `config.Stream()` will populate the provided channel with results. Within this, there are two channels within each
`Result`, `StdOutStream` and `StdErrStream`, which hold the `stdout` and `stderr` pipes respectively. Reading from these
channels will give you the host's output/errors. 

When a host has completed it's work and has exited, `Result.DoneChannel` will receive an empty struct. In my example, I use
the following function to monitor this and report that a host has finished (see `_examples/example_streaming` for full program);

```go
func readStream(res Result, wg *sync.WaitGroup) error {
	for {
		select {
		case d := <-res.StdOutStream:
			fmt.Printf("%s: %s", res.Host, d)
		case <-res.DoneChannel:
			fmt.Printf("%s: Finished\n", res.Host)
			wg.Done()
		}
	}
}
```

Unlike with `Config.Run()`, which returns a slice of `Result`s when all hosts have exited, `Config.Stream()` requires some
additional values to monitor host completion. For each individual host we have `Result.DoneChannel`, as explained above, but
to detect when _all_ hosts have finished, we have the variable `NumberOfStreamingHostsCompleted`, which will equal the length 
of `Config.Hosts` once everything has completed. Here is an example of what I'm using in `_examples/example_streaming`;

```go
if NumberOfStreamingHostsCompleted == len(cfg.Hosts) {
		// We want to wait for all goroutines to complete before we declare that the work is finished, as
		// it's possible for us to execute this code before we've finished reading/processing all host output
		wg.Wait()

		fmt.Println("Everything returned.")
		return
}
```

Right now, the concurrency model used to read from the results channel is the responsibility of those using this package. An example of
how this might be achieved can be found in the https://github.com/DiscoRiver/omnivore/tree/main/internal/ossh package, which is currently in development.

