package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"syscall"
)

func readPassword(prompt string) (ssh.AuthMethod, error) {
	fmt.Fprint(os.Stderr, prompt)
	var fd int
	if terminal.IsTerminal(syscall.Stdin) {
		fd = syscall.Stdin
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return nil, fmt.Errorf("Failed to open '/dev/tty': %s", err)
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}
	bytePassword, err := terminal.ReadPassword(fd)
	if err != nil {
		return nil, fmt.Errorf("Failed to read password: %s", err)
	}
	fmt.Fprintln(os.Stderr)
	return ssh.Password(string(bytePassword)), nil
}

