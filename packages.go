package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/reconquest/executil-go"
)

func installPackages(target string, packages []string) error {
	args := []string{"-c", "-d", target}
	command := exec.Command("pacstrap", append(args, packages...)...)

	command.Stdout = os.Stderr
	command.Stderr = os.Stderr

	_, _, err := executil.Run(
		command,
		executil.IgnoreStderr,
		executil.IgnoreStdout,
	)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(target, ".packages"), []byte(
		strings.Join(packages, "\n"),
	), 0644)

	return err
}

func listExplicitlyInstalled(baseDir string) ([]string, error) {
	rawPackages, err := ioutil.ReadFile(filepath.Join(baseDir, ".packages"))
	if err != nil {
		return nil, err
	}

	packages := strings.Split(strings.TrimSpace(string(rawPackages)), "\n")

	return packages, nil
}
