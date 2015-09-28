package main

import (
	"fmt"
	"os/exec"
)

func formatExecError(command *exec.Cmd, err error, output []byte) error {
	return fmt.Errorf("exec %q failed (%s):\n%s", command.Args, err, output)
}

func formatAbsPathError(relative string, err error) error {
	return fmt.Errorf("can't get abs path for '%s': %s", relative, err)
}
