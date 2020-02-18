package main

import (
	"flag"
	"os"
)

var command cmdEnv
type cmdEnv struct {
	Hosts []string
	Script string
	WorkerPool int
	User string
	Timeout int
}

func parseCommands() {


	command.Script = *flag.String("s", "", "Path to script file.")
	command.WorkerPool = *flag.Int("w", 5, "Specify amount of concurrent workers.")
	command.User = *flag.String("u", "", "Specify user for ssh.")
	command.Timeout = *flag.Int("t", 10, "Timeout for ssh.")

	if len(os.Args) < 2 {
		flag.Usage()
	}

	flag.Parse()
}