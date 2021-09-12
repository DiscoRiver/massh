![logo](./doc/logo.jpg)

[![Go Report Card](https://goreportcard.com/badge/github.com/DiscoRiver/massh)](https://goreportcard.com/report/github.com/DiscoRiver/massh) ![Go Report Card](https://img.shields.io/github/license/DiscoRiver/massh) [![Go Doc](https://img.shields.io/badge/GoDoc-Available-informational)](https://godoc.org/github.com/DiscoRiver/massh)

## Description
Go package for running Linux distributed shell commands via SSH. 

## Example:

```
package main

import "github.com/discoriver/massh"

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
	
        // Make sure config will run
        config.CheckSanity()

	config.Run()
}
```

More examples available in the examples directory.

## Usage:
Get the massh package;

```go get github.com/DiscoRiver/massh/massh```

## Documentation

* [GoDoc](https://godoc.org/github.com/DiscoRiver/massh/massh)

## Other

### Bastion Host

It's possible use massh with a Bastion host. You do this by specifying `BastionHost` and `BastionHostSSHConfig` in 
`Config`. You may leave `BastionHostSSHConfig` as `nil`, in which case `SSHConfig` will be used instead. The process is
automatic, and if `BastionHost` is not `nil`, it will be used. 

### Streaming output

There is an example of streaming output in the direcotry `_examples/example_streaming`, which contains one way of reading
from the results channel, and processing the output.

Running `config.Stream()` will populate the provided channel with results. Within this, there are two channels within each
`Result`, `StdOutStream` and `StdErrStream`, which hold the `stdout` and `stderr` pipes respectively. Reading from these
channels will give you the host's output/errors. 

When a host has completed it's work and has exited, `Result.DoneChannel` will receive an empty struct. In my example, I use
the following function to monitor this and report that a host has finished (see `_examples/example_streaming` for full program);

```
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

```
if NumberOfStreamingHostsCompleted == len(cfg.Hosts) {
		// We want to wait for all goroutines to complete before we declare that the work is finished, as
		// it's possible for us to execute this code before we've finished reading/processing all host output
		wg.Wait()

		fmt.Println("Everything returned.")
		return
}
```

Ultimately, the concurrency model used to read from the results channel is the responsibility of those using this package. 

