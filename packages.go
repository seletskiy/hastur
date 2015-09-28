package main

import "io/ioutil"
import "os/exec"
import "path/filepath"
import "strings"

func installPackages(target string, packages []string) error {
	args := []string{"-d", target}
	command := exec.Command("pacstrap", append(args, packages...)...)

	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
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
