package massh

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type script interface {
	prepare(*ssh.Session)
	getPreparedCommandString() string
	getBytes() []byte
	getArgs() string
}

type shell struct {
	bytes []byte
	args  string

	commandString string

	prepared bool
}

type python struct {
	bytes []byte
	args  string

	commandString string

	prepared bool
}

// NewScript creates a new script type based on the file extension. Shebang line in supported scripts must be present.
//
// Each element in args should ideally contain an argument's key/value, for example "--some-arg value", or "--some-arg=value".
func newScript(scriptFile string, args ...string) (script, error) {
	scriptBytes, err := ioutil.ReadFile(scriptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file: %s", err)
	}

	// Check shebang is present
	if scriptBytes[0] != '#' {
		return nil, fmt.Errorf("shebang line not present in file %s", filepath.Base(scriptFile))
	}

	if strings.HasSuffix(scriptFile, ".sh") {
		shellScript := &shell{
			bytes: scriptBytes,
			args:  strings.Join(args, " "),
		}
		return shellScript, nil
	}

	if strings.HasSuffix(scriptFile, ".py") {
		pythonScript := &python{
			bytes: scriptBytes,
			args:  strings.Join(args, " "),
		}
		return pythonScript, nil
	}

	return nil, fmt.Errorf("script file %s not supported", filepath.Base(scriptFile))
}

// Prepare populated the SSH sessions's stdin with the script data, and returns a command string to run the script from a temporary file.
func (s *shell) prepare(session *ssh.Session) {
	// Set up remote script
	session.Stdin = bytes.NewReader(s.bytes)

	s.commandString = fmt.Sprintf("cat > massh-script-tmp.sh && chmod +x ./massh-script-tmp.sh && ./massh-script-tmp.sh %s && rm ./massh-script-tmp.sh", s.args)
	s.prepared = true
}

func (s *shell) getPreparedCommandString() string {
	return s.commandString
}

func (s *shell) getBytes() []byte {
	return s.bytes
}

func (s *shell) getArgs() string {
	return s.args
}

// Prepare populated the SSH sessions's stdin with the script data, and returns a command string to run the script from a temporary file.
func (s *python) prepare(session *ssh.Session) {
	// Set up remote script
	session.Stdin = bytes.NewReader(s.bytes)

	s.commandString = fmt.Sprintf("cat > massh-script-tmp.py && chmod +x ./massh-script-tmp.py && ./massh-script-tmp.py %s && rm ./massh-script-tmp.py", s.args)
	s.prepared = true
}

func (s *python) getPreparedCommandString() string {
	return s.commandString
}

func (s *python) getBytes() []byte {
	return s.bytes
}

func (s *python) getArgs() string {
	return s.args
}
