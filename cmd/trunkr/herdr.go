package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// herdr invokes the running herdr binary (via HERDR_BIN_PATH) with the given
// args, inheriting the plugin environment so socket routing just works.
func herdr(args ...string) (string, error) {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		return "", fmt.Errorf("HERDR_BIN_PATH is not set; trunkr must run as a herdr plugin command")
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
