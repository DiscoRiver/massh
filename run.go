package massh

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ErrSlow is the error for jobs stopped because they were slow (see Config.CancelSlow).
var ErrSlow = errors.New("massh: job stopped after slow timeout")

// Stream starts running c's jobs on every host and returns a channel of events as they happen.
//
// For every host and job, Stream sends zero or more EventStdout and EventStderr events, at most one
// EventSlow event, and finally one EventDone event. Events for different hosts are interleaved.
// The channel is closed once every host has finished.
//
// The caller must keep receiving from the channel until it is closed, or cancel ctx; otherwise
// the remaining work blocks. Once ctx is cancelled, remaining events may be dropped, and the
// channel is closed as soon as all connections have been torn down.
//
// Stream returns an error without starting any work if c is invalid, or if it can't connect to
// the bastion host.
func (c *Config) Stream(ctx context.Context) (<-chan Event, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	r := &runner{
		ctx:              ctx,
		hosts:            c.uniqueHosts(),
		jobs:             slices.Clone(c.Jobs),
		sshConfig:        c.SSHConfig,
		bastionHost:      c.BastionHost,
		bastionSSHConfig: c.bastionSSHConfig(),
		resolve:          c.Resolve,
		port:             c.port(),
		workers:          c.workers(),
		slowTimeout:      c.SlowTimeout,
		cancelSlow:       c.CancelSlow,
		bastions:         make(map[bastionKey]*bastionConn),
	}

	// Connect to the default bastion now, so that a problem with it is reported here.
	if c.BastionHost != "" {
		if _, err := r.bastion(c.BastionHost, r.bastionSSHConfig); err != nil {
			return nil, fmt.Errorf("massh: bastion: %w", err)
		}
	}

	events := make(chan Event)
	go r.run(events)
	return events, nil
}

// Run runs c's jobs on every host and waits for them all to finish. It returns one Result for each
// host and job, ordered by host (in the order of c.Hosts) and then by job.
//
// Failures on individual hosts are reported in Result.Err. Run returns an error only if no work
// could be started (see [Config.Stream]), or ctx was cancelled; in the latter case the Results
// are still returned, and those that didn't finish have ctx's error.
//
// Run keeps all output in memory; use [Config.Stream] for large outputs or many hosts.
func (c *Config) Run(ctx context.Context) ([]Result, error) {
	events, err := c.Stream(ctx)
	if err != nil {
		return nil, err
	}

	hosts := c.uniqueHosts()
	results := make([]Result, 0, len(hosts)*len(c.Jobs))
	type key struct {
		host string
		job  int
	}
	index := make(map[key]int, cap(results))
	for _, h := range hosts {
		for j, job := range c.Jobs {
			index[key{h, j}] = len(results)
			results = append(results, Result{Host: h, Job: j, Command: job.Command})
		}
	}

	done := make([]bool, len(results))
	for ev := range events {
		i := index[key{ev.Host, ev.Job}]
		r := &results[i]
		switch ev.Kind {
		case EventStdout:
			r.Stdout = append(r.Stdout, ev.Data...)
		case EventStderr:
			r.Stderr = append(r.Stderr, ev.Data...)
		case EventSlow:
			r.Slow = true
		case EventDone:
			r.Err = ev.Err
			done[i] = true
		}
	}

	if err := ctx.Err(); err != nil {
		for i := range results {
			if !done[i] {
				results[i].Err = err
			}
		}
		return results, err
	}
	return results, nil
}

// runner holds the state of one call to Stream.
type runner struct {
	ctx              context.Context
	hosts            []string
	jobs             []Job
	sshConfig        *ssh.ClientConfig
	bastionHost      string
	bastionSSHConfig *ssh.ClientConfig
	resolve          func(string) (HostOptions, error)
	port             int
	workers          int
	slowTimeout      time.Duration
	cancelSlow       bool

	mu       sync.Mutex
	bastions map[bastionKey]*bastionConn
}

type bastionKey struct{ addr, user string }

// bastionConn is a connection to a bastion host, shared by every host that uses it.
type bastionConn struct {
	once   sync.Once
	client *ssh.Client
	err    error
}

// bastion returns the shared connection to the bastion host, connecting on first use.
func (r *runner) bastion(host string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	addr := hostAddr(host, r.port)
	key := bastionKey{addr, cfg.User}
	r.mu.Lock()
	b, ok := r.bastions[key]
	if !ok {
		b = &bastionConn{}
		r.bastions[key] = b
	}
	r.mu.Unlock()
	b.once.Do(func() { b.client, b.err = dial(r.ctx, nil, addr, cfg) })
	return b.client, b.err
}

// connect resolves host's options and connects to it.
func (r *runner) connect(host string) (*ssh.Client, error) {
	var opts HostOptions
	if r.resolve != nil {
		var err error
		if opts, err = r.resolve(host); err != nil {
			return nil, fmt.Errorf("massh: resolve %s: %w", host, err)
		}
	}
	cfg := cmp.Or(opts.SSHConfig, r.sshConfig)
	if err := validateSSHConfig(cfg); err != nil {
		return nil, err
	}

	bastionHost, bastionCfg := r.bastionHost, r.bastionSSHConfig
	if opts.Bastion != "" && opts.Bastion != r.bastionHost {
		bastionHost, bastionCfg = opts.Bastion, cfg
	}
	if opts.BastionSSHConfig != nil {
		bastionCfg = opts.BastionSSHConfig
	}
	var bastion *ssh.Client
	if bastionHost != "" {
		if err := validateSSHConfig(bastionCfg); err != nil {
			return nil, fmt.Errorf("massh: bastion %s: %w", bastionHost, err)
		}
		var err error
		if bastion, err = r.bastion(bastionHost, bastionCfg); err != nil {
			return nil, fmt.Errorf("massh: bastion %s: %w", bastionHost, err)
		}
	}
	return dial(r.ctx, bastion, hostAddr(cmp.Or(opts.Addr, host), r.port), cfg)
}

func (r *runner) run(events chan<- Event) {
	defer close(events)
	defer func() {
		for _, b := range r.bastions {
			if b.client != nil {
				b.client.Close()
			}
		}
	}()

	hosts := make(chan string)
	var wg sync.WaitGroup
	for range min(r.workers, len(r.hosts)) {
		wg.Go(func() {
			for h := range hosts {
				r.runHost(h, events)
			}
		})
	}

feed:
	for _, h := range r.hosts {
		select {
		case hosts <- h:
		case <-r.ctx.Done():
			break feed
		}
	}
	close(hosts)
	wg.Wait()
}

// emit sends ev unless the work has been cancelled.
func (r *runner) emit(events chan<- Event, ev Event) {
	select {
	case events <- ev:
	case <-r.ctx.Done():
	}
}

// runHost connects to host and runs every job on it in turn.
func (r *runner) runHost(host string, events chan<- Event) {
	client, err := r.connect(host)
	if err != nil {
		for i, job := range r.jobs {
			r.emit(events, Event{Kind: EventDone, Host: host, Job: i, Command: job.Command, Err: err})
		}
		return
	}
	defer client.Close()
	// Closing the connection is the only reliable way to interrupt a running command.
	stop := context.AfterFunc(r.ctx, func() { client.Close() })
	defer stop()

	for i, job := range r.jobs {
		ev := Event{Host: host, Job: i, Command: job.Command}
		err := r.runJob(client, job, ev, events)
		if err != nil && r.ctx.Err() != nil {
			err = r.ctx.Err()
		}
		ev.Kind, ev.Err = EventDone, err
		r.emit(events, ev)
	}
}

// runJob runs job in a new session on client, sending its output as events based on base.
func (r *runner) runJob(client *ssh.Client, job Job, base Event, events chan<- Event) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("massh: new session: %w", err)
	}
	defer sess.Close()

	if job.Stdin != nil {
		sess.Stdin = bytes.NewReader(job.Stdin)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("massh: stdout: %w", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return fmt.Errorf("massh: stderr: %w", err)
	}
	if err := sess.Start(job.Command); err != nil {
		return fmt.Errorf("massh: start: %w", err)
	}

	// Both pipes must be read concurrently, or a command that fills one while we read the other
	// would stall. Lines from both are funnelled through one channel so that only this goroutine
	// sends events for the job, which keeps them ordered and guarantees EventDone comes last.
	lines := make(chan Event)
	var readers sync.WaitGroup
	readers.Go(func() { readLines(stdout, EventStdout, base, lines) })
	readers.Go(func() { readLines(stderr, EventStderr, base, lines) })
	go func() {
		readers.Wait()
		close(lines)
	}()

	var slowC <-chan time.Time
	var slowTimer *time.Timer
	if r.slowTimeout > 0 {
		slowTimer = time.NewTimer(r.slowTimeout)
		defer slowTimer.Stop()
		slowC = slowTimer.C
	}
	slow := false

	// Keep draining lines even after cancellation, so the readers can always finish.
	for lines != nil {
		select {
		case ev, ok := <-lines:
			if !ok {
				lines = nil
				continue
			}
			r.emit(events, ev)
			if slowC != nil {
				slowTimer.Reset(r.slowTimeout)
			}
		case <-slowC:
			slow, slowC = true, nil
			ev := base
			ev.Kind = EventSlow
			r.emit(events, ev)
			if r.cancelSlow {
				_ = sess.Signal(ssh.SIGKILL)
				_ = sess.Close()
			}
		}
	}

	err = sess.Wait()
	if slow && r.cancelSlow {
		return ErrSlow
	}
	return err
}

// MaxLineLength is the longest line sent in one event. Longer lines are split into several
// events, so that a host writing endless output without newlines can't exhaust memory.
const MaxLineLength = 64 << 10

// readLines sends each line read from rd to lines as an event of the given kind.
func readLines(rd io.Reader, kind EventKind, base Event, lines chan<- Event) {
	br := bufio.NewReaderSize(rd, MaxLineLength)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			ev := base
			// ReadSlice's result is only valid until the next read.
			ev.Kind, ev.Data = kind, bytes.Clone(line)
			lines <- ev
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			// io.EOF, or the connection failed, which sess.Wait reports.
			return
		}
	}
}
