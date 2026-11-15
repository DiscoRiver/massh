# Changelog

## v2.0.0 (unreleased)

v2 is a rewrite around `context.Context` and a single channel of events. The module path is now
`github.com/discoriver/massh/v2`; v1 users are unaffected until they change their import path.

### Breaking changes
- `Config.Run(ctx)` and `Config.Stream(ctx)` take a context. Cancelling it closes every
  connection; it replaces `Config.Stop` and `StopAllSessions`.
- `Config.Stream` returns a `<-chan Event` that is closed when every host has finished. Each event
  is a line of stdout or stderr, a slow notice, or the final `EventDone` (with the job's error) for a
  host and job. This replaces the per-host `StdOutStream`/`StdErrStream`/`DoneChannel` channels and the
  global `NumberOfStreamingHostsCompleted` counter.
- `Job` and `JobStack` are replaced by `Jobs []Job`. Jobs run in order over one connection per host.
- `Hosts` is a `[]string` (duplicates are still ignored, and order is kept). Hosts may include a port.
- `Result` has separate `Stdout` and `Stderr`, and keeps the output of commands that exit non-zero.
- `WorkerPool` is now `Workers`, and defaults to `DefaultWorkers` instead of being required.
- `SlowTimeout` is a `time.Duration`; zero disables slow detection (it used to flag every host).
- `CancelSlowHosts` is now `CancelSlow`, and is implemented: slow jobs are stopped with `ErrSlow`.
- The `Set*` helpers are gone; set `Config` fields directly. Auth helpers are now the functions
  `PrivateKeyAuth`, `AgentAuth`, the new `PrivateKeySigner`/`AgentSigners` (to combine keys
  from several sources in one auth method), and the new `KnownHosts` host key callback.
- `Job.SetScript` is now `NewScriptJob`. Any script with a shebang is accepted, each argument is
  passed as one shell-quoted word, and the temporary file is removed even if the script fails.
- `CheckSanity` is now `Validate`, which returns all problems joined into one error.

### Fixes
- Stdout and stderr are read concurrently; a command writing a lot to stderr no longer hangs.
- The final line of output is no longer dropped when it has no trailing newline.
- No more data races or goroutine leaks (the completion counter, slow-timer goroutines, stop
  watchers and output readers are gone or bounded).
- `BastionSSHConfig` is used when set, and `SSHConfig` when it isn't (previously `NewConfig`
  always set an empty bastion config). One bastion connection is shared and closed afterwards.
- Job stacks keep `SlowTimeout` and cancellation.
- `ssh.ClientConfig.Timeout` also bounds the SSH handshake, and the bastion hop.

### Other
- Requires Go 1.26 and the current `golang.org/x/crypto`. `go-homedir` is no longer used.
- New `sshtest` package: an in-process SSH server for hermetic tests. The test suite no longer
  needs a local sshd.

## v1

26/12/2021
- Added slow host detection. This change BREAKS any call to Config.Stream() in previous versions, as the Result value is now accepted as a pointer.

29/09/2021
- Minor error and comment updates.

25/09/2021
- Changed the value of hosts in the config to a map, for the purpose of not allowing duplicate hosts.

18/09/2021
- Updated Config.SetPublicKeyAuth to accept a key passphrase. Will break existing uses of this function. Use v1.4.0 to avoid this change temporarily.

24/07/2021
- Added JobStack to config. Multiple jobs are process automatically and do NOT reuse and active session. They return separate results, meaning you would have multiple `Result` types with the same host, but a different job string. Both `Job` and `JobStack` cannot be present when executing `Run()` or `Stream()`