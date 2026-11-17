package massh

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// EventKind says what an Event reports.
type EventKind int

const (
	// EventStdout carries one line of the job's standard output in Event.Data.
	EventStdout EventKind = iota + 1
	// EventStderr carries one line of the job's standard error in Event.Data.
	EventStderr
	// EventSlow reports that the job has produced no output for Config.SlowTimeout. It is sent at
	// most once per host and job, and the job keeps running unless Config.CancelSlow is set.
	EventSlow
	// EventDone is the last event for a host and job. Event.Err is nil if the job succeeded.
	EventDone
)

func (k EventKind) String() string {
	switch k {
	case EventStdout:
		return "stdout"
	case EventStderr:
		return "stderr"
	case EventSlow:
		return "slow"
	case EventDone:
		return "done"
	default:
		return fmt.Sprintf("EventKind(%d)", int(k))
	}
}

// Event is something that happened while running a job on a host. See [Config.Stream].
type Event struct {
	Kind EventKind

	Host string
	// Job is the index of the job in Config.Jobs.
	Job int
	// Command is the job's command.
	Command string

	// Data is a line of output, including its trailing newline if it had one. Lines longer than
	// MaxLineLength are split across several events. It is set for EventStdout and EventStderr,
	// and is never modified after the event is sent.
	Data []byte

	// Err is set for EventDone if the job failed. Use [ExitStatus] to get the exit status of a
	// command that ran but failed (an *ssh.ExitError, or an *ExitError with OpenSSH). With
	// OpenSSH, a failure of ssh itself is an *OpenSSHError. If the work was cancelled, Err is
	// the context's error.
	Err error
}

// Result is the outcome of one job on one host. See [Config.Run].
type Result struct {
	Host string
	// Job is the index of the job in Config.Jobs.
	Job     int
	Command string

	Stdout []byte
	Stderr []byte

	// Slow is true if the job was flagged slow (see Config.SlowTimeout).
	Slow bool

	// Err is nil if the job succeeded. See Event.Err.
	Err error
}

// ExitStatus returns the exit status of a job's command from its error, and whether there is
// one: it is false for nil and for errors that aren't about the command's exit status, such as
// connection failures.
func ExitStatus(err error) (int, bool) {
	if e, ok := errors.AsType[*ssh.ExitError](err); ok {
		return e.ExitStatus(), true
	}
	if e, ok := errors.AsType[*ExitError](err); ok {
		return e.Status, true
	}
	return 0, false
}
