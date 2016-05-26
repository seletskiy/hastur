package main

import "os/exec"

func addPostroutingMasquarading(dev string) error {
	args := []string{"-t", "nat", "-A", "POSTROUTING", "-o", dev,
		"-j", "MASQUERADE"}

	command := exec.Command("iptables", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func removePostroutingMasquarading(dev string) error {
	args := []string{"-t", "nat", "-D", "POSTROUTING", "-o", dev,
		"-j", "MASQUERADE"}

	command := exec.Command("iptables", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}
