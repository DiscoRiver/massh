package massh_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/discoriver/massh/v2"
	"golang.org/x/crypto/ssh"
)

func Example() {
	key, err := massh.PrivateKeyAuth("~/.ssh/id_ed25519", "")
	if err != nil {
		log.Fatal(err)
	}
	hostKeys, err := massh.KnownHosts()
	if err != nil {
		log.Fatal(err)
	}

	cfg := &massh.Config{
		Hosts: []string{"web1.example.com", "web2.example.com:2222"},
		Jobs:  []massh.Job{{Command: "uptime"}},
		SSHConfig: &ssh.ClientConfig{
			User:            "deploy",
			Auth:            []ssh.AuthMethod{key},
			HostKeyCallback: hostKeys,
			Timeout:         10 * time.Second,
		},
	}

	results, err := cfg.Run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("%s: error: %v\n", r.Host, r.Err)
			continue
		}
		fmt.Printf("%s: %s", r.Host, r.Stdout)
	}
}

func ExampleConfig_Stream() {
	agent, err := massh.AgentAuth()
	if err != nil {
		log.Fatal(err)
	}
	hostKeys, err := massh.KnownHosts()
	if err != nil {
		log.Fatal(err)
	}

	cfg := &massh.Config{
		Hosts:       []string{"db1", "db2", "db3"},
		Jobs:        []massh.Job{{Command: "tail -f /var/log/syslog"}},
		BastionHost: "bastion.example.com",
		SSHConfig: &ssh.ClientConfig{
			User:            "ops",
			Auth:            []ssh.AuthMethod{agent},
			HostKeyCallback: hostKeys,
		},
		SlowTimeout: 30 * time.Second,
	}

	// Stop everything on Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	events, err := cfg.Stream(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for ev := range events {
		switch ev.Kind {
		case massh.EventStdout, massh.EventStderr:
			fmt.Printf("%s: %s", ev.Host, ev.Data)
		case massh.EventSlow:
			fmt.Printf("%s: no output for 30s\n", ev.Host)
		case massh.EventDone:
			fmt.Printf("%s: finished, err=%v\n", ev.Host, ev.Err)
		}
	}
}

func ExampleNewScriptJob() {
	job, err := massh.NewScriptJob("./check-disk.sh", "--threshold", "90")
	if err != nil {
		log.Fatal(err)
	}
	cfg := &massh.Config{
		Hosts: []string{"web1", "web2"},
		Jobs:  []massh.Job{job},
		// SSHConfig: ...
	}
	_ = cfg
}
