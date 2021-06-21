package massh

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
)

// getJob determines the type of job and returns the command string
func getJob(s *ssh.Session, j *Job) string {
	// Set up remote script
	if j.Script != nil {
		s.Stdin = bytes.NewReader(j.Script)
		return fmt.Sprintf("cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh %s && rm ./outfile.sh", j.ScriptArgs)
	}

	return j.Command
}
