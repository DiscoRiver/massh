package massh

import (
	"bytes"
	"golang.org/x/crypto/ssh"
)

// getJob determines the type of job and returns the command string
func getJob(s *ssh.Session, j *Job) string {
	// Set up remote script
	if j.script != nil {
		s.Stdin = bytes.NewReader(j.script)
		return "cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh && rm ./outfile.sh"
	}

	return j.Command
}
