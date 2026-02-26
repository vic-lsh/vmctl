package internal

import (
	"fmt"
	"os/exec"
)

// SSHRun executes a command on a remote host via SSH.
func SSHRun(user, ip, command string) error {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", user, ip),
		command,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh %s@%s: %w\n%s", user, ip, err, string(out))
	}
	fmt.Print(string(out))
	return nil
}
