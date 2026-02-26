package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteVMInfo writes the .vm metadata file to the VM path.
func WriteVMInfo(vmPath, name, username string) error {
	content := fmt.Sprintf("NAME=%s\nUSERNAME=%s\n", name, username)
	return os.WriteFile(filepath.Join(vmPath, ".vm"), []byte(content), 0644)
}

// ReadVMInfo reads the .vm metadata file and returns the VM name and username.
func ReadVMInfo(vmPath string) (name, username string, err error) {
	data, err := os.ReadFile(filepath.Join(vmPath, ".vm"))
	if err != nil {
		return "", "", fmt.Errorf("no VM info found at %s/.vm: %w", vmPath, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			switch k {
			case "NAME":
				name = v
			case "USERNAME":
				username = v
			}
		}
	}

	if name == "" || username == "" {
		return "", "", fmt.Errorf("incomplete VM info in %s/.vm", vmPath)
	}
	return name, username, nil
}
