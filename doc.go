// Package massh runs shell commands on many hosts concurrently over SSH.
//
// Describe the work with a [Config]: the hosts, the jobs to run on each of them, and an
// [ssh.ClientConfig] used to connect. Then either
//
//   - call [Config.Run] to wait for every host and get one [Result] per host and job, or
//   - call [Config.Stream] to receive an [Event] for every line of output as it is produced,
//     followed by an [EventDone] event for each host and job.
//
// Both take a [context.Context]; cancelling it closes every connection and stops all work.
//
// Each host gets a single SSH connection (optionally tunnelled through a bastion host), and its
// jobs run one after another, each in a new session on that connection. At most
// [Config.Workers] hosts are processed at the same time.
//
// [PrivateKeyAuth], [AgentAuth] and [KnownHosts] help build the [ssh.ClientConfig].
package massh
