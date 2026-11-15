![logo](./doc/logo.jpg)

[![Go Test](https://github.com/DiscoRiver/massh/actions/workflows/go-test.yml/badge.svg)](https://github.com/DiscoRiver/massh/actions/workflows/go-test.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/DiscoRiver/massh)](https://goreportcard.com/report/github.com/DiscoRiver/massh)  ![License](https://img.shields.io/github/license/DiscoRiver/massh) [![Go Reference](https://pkg.go.dev/badge/github.com/discoriver/massh/v2.svg)](https://pkg.go.dev/github.com/discoriver/massh/v2)

## Description

Go package for running shell commands on many Linux hosts at once over SSH.

What makes Massh special is its ability to stream and process output from every host concurrently,
as it is produced.

```
go get github.com/discoriver/massh/v2
```

v2 changed the API substantially; see the [changelog](CHANGELOG.md). v1 remains available at
`github.com/discoriver/massh`.

## Example

```go
key, err := massh.PrivateKeyAuth("~/.ssh/id_ed25519", "")
if err != nil {
	log.Fatal(err)
}
hostKeys, err := massh.KnownHosts() // ~/.ssh/known_hosts
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

results, err := cfg.Run(ctx)
if err != nil {
	log.Fatal(err)
}
for _, r := range results {
	fmt.Printf("%s: %s (err=%v)\n", r.Host, r.Stdout, r.Err)
}
```

## Streaming output

`Config.Stream` returns a channel of events, closed once every host has finished:

```go
events, err := cfg.Stream(ctx)
if err != nil {
	log.Fatal(err)
}
for ev := range events {
	switch ev.Kind {
	case massh.EventStdout, massh.EventStderr:
		fmt.Printf("%s: %s", ev.Host, ev.Data)
	case massh.EventSlow:
		fmt.Printf("%s: no output for a while\n", ev.Host)
	case massh.EventDone:
		fmt.Printf("%s: finished, err=%v\n", ev.Host, ev.Err)
	}
}
```

Each host and job gets zero or more output events (one per line), at most one `EventSlow`, and
exactly one final `EventDone`. Keep reading until the channel is closed, or cancel the context to
stop all work: cancelling closes every connection.

## Other features

- **Multiple jobs**: `Jobs` run in order on each host, over a single connection.
- **Scripts**: `massh.NewScriptJob("./script.sh", "arg")` copies a local script to each host,
  runs it, and removes it again.
- **Bastion host**: set `BastionHost` (and optionally `BastionSSHConfig`) to tunnel every
  connection through a jump host.
- **System ssh client**: set `OpenSSH` to run your `ssh` instead of the Go SSH library, so
  everything in `~/.ssh/config` works, along with Kerberos, security keys and certificates. massh
  hardens it: hosts can't be read as options, forwarding is off by default, and it never prompts.
- **Per-host settings**: `Resolve` can give each host its own address, user, keys and jump host,
  for example from `~/.ssh/config`.
- **Host keys**: `KnownHosts` checks `~/.ssh/known_hosts`; `KnownHostsAcceptNew` also records
  keys of new hosts, like `StrictHostKeyChecking accept-new`.
- **Concurrency**: `Workers` limits how many hosts are processed at once (default 32).
- **Slow hosts**: `SlowTimeout` flags jobs that go quiet; `CancelSlow` stops them.
- **Testing**: the `sshtest` package provides an in-process SSH server, like `httptest`.

## Documentation

* [Go reference](https://pkg.go.dev/github.com/discoriver/massh/v2)

## Contribute

Have a question, idea, or something you think can be improved? Open an issue or PR and let's
discuss it!
