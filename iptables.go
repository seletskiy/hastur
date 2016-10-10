package main

import (
	"os/exec"

	"github.com/reconquest/executil-go"
)

func addPostroutingMasquarading(dev string) error {
	args := []string{"-t", "nat", "-A", "POSTROUTING", "-o", dev,
		"-j", "MASQUERADE"}

	command := exec.Command("iptables", args...)
	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func removePostroutingMasquarading(dev string) error {
	args := []string{"-t", "nat", "-D", "POSTROUTING", "-o", dev,
		"-j", "MASQUERADE"}

	command := exec.Command("iptables", args...)
	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}
