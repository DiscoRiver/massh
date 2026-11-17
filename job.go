package massh

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// Job is a command to run on every host.
type Job struct {
	// Command is run by the remote user's login shell.
	Command string

	// Stdin, if non-nil, is sent to the command's standard input.
	Stdin []byte
}

// NewScriptJob returns a Job that copies the local script at path to each host, runs it with args,
// and removes it again. The script must start with a shebang line ("#!"), which selects the
// interpreter on the remote host.
//
// Each element of args is passed to the script as a single, shell-quoted argument. The script is
// written to a temporary file in the remote user's working directory (normally their home
// directory), and the remote login shell must be POSIX compatible.
func NewScriptJob(path string, args ...string) (Job, error) {
	script, err := os.ReadFile(path)
	if err != nil {
		return Job{}, fmt.Errorf("massh: read script: %w", err)
	}
	if !bytes.HasPrefix(script, []byte("#!")) {
		return Job{}, fmt.Errorf("massh: script %s has no shebang line", path)
	}

	var cmd strings.Builder
	// The EXIT trap removes the script whatever happens, and keeps the script's exit status.
	cmd.WriteString(`f=$(mktemp ./.massh-script.XXXXXX) && trap 'rm -f "$f"' EXIT && cat > "$f" && chmod 700 "$f" && "$f"`)
	for _, a := range args {
		cmd.WriteByte(' ')
		cmd.WriteString(shellQuote(a))
	}

	return Job{Command: cmd.String(), Stdin: script}, nil
}

// shellQuote quotes s as a single word for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
