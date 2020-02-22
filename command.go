package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

var command cmdEnv
type cmdEnv struct {
	Hosts []string
	Script string
	ScriptArgs string
	WorkerPool int
	User string
	Timeout int
	PublicKey string
	Insecure bool
	Command string
}

func parseCommands() {
	flag.StringVar(&command.Script,"s", "", "Path to script file. Overrides -c switch.")
	flag.StringVar(&command.ScriptArgs, "a", "", "Arguments for script")
	flag.IntVar(&command.WorkerPool,"w", 5, "Specify amount of concurrent workers.")
	flag.StringVar(&command.User,"u", "", "Specify user for ssh.")
	flag.IntVar(&command.Timeout,"t", 10, "Timeout for ssh.")
	flag.StringVar(&command.PublicKey, "p", "", "Public key file.")
	flag.BoolVar(&command.Insecure, "insecure", false, "Set insecure key mode.")
	flag.StringVar(&command.Command, "c", "", "Set remote command to run.")

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(0)
	}

	flag.Parse()
	parseHosts()
}

func parseHosts() {
	CheckStdin()
	reader := bufio.NewReader(os.Stdin)

	for {
		input, err := reader.ReadString('\n')
		if err != nil && err == io.EOF {
			if len(command.Hosts) == 0 {
				fmt.Println("no hosts provided")
				os.Exit(1)
			} else {
				return
			}
		}
		command.Hosts = append(command.Hosts, strings.Trim(input, "\n"))
		}
}

// CheckStdin ensures os.Stdin is available, and checks the pipe for potential errors.
func CheckStdin() {
	stdin, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Could not read stdin: %s", err)
		os.Exit(1)
	}

	// Make sure pipe is good
	if stdin.Mode()&os.ModeCharDevice != 0 {
		fmt.Println("bad pipe or no hosts provided:", stdin.Size())
		os.Exit(1)
	}
}