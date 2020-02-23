package massh

import (
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"syscall"
)

// ReadPassword prompts user for their password, with the provided prompt string.
func ReadPassword(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	var fd int
	if terminal.IsTerminal(syscall.Stdin) {
		fd = syscall.Stdin
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return nil, fmt.Errorf("Failed to open terminal: %s", err)
		}
		defer tty.Close()
		fd = int(tty.Fd())
	}
	bytePassword, err := terminal.ReadPassword(fd)
	if err != nil {
		return nil, fmt.Errorf("Failed to read password: %s", err)
	}
	fmt.Fprintln(os.Stderr)

	return bytePassword, nil
}
