package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func listActiveContainers(
	containerSuffix string,
) (map[string]struct{}, error) {
	command := exec.Command("machinectl", "--no-legend")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, formatExecError(command, err, output)
	}

	containers := map[string]struct{}{}
	rawContainers := strings.Split(string(output), "\n")

	for _, rawContainer := range rawContainers {
		if rawContainer == "" {
			continue
		}

		fields := strings.Fields(rawContainer)
		if len(fields) < 3 {
			return nil, fmt.Errorf(
				"invalid output from machinectl: %s", rawContainer,
			)
		}

		if strings.HasSuffix(fields[0], containerSuffix) {
			nameWithoutSuffix := strings.TrimSuffix(fields[0], containerSuffix)
			containers[nameWithoutSuffix] = struct{}{}
		}
	}

	return containers, nil
}

func getContainerLeaderPID(name string) (int, error) {
	command := exec.Command("machinectl", "show", name+containerSuffix)
	output, err := command.CombinedOutput()
	if err != nil {
		return 0, formatExecError(command, err, output)
	}

	config := strings.Split(string(output), "\n")
	for _, line := range config {
		if strings.HasPrefix(line, "Leader=") {
			pid, err := strconv.Atoi(strings.Split(line, "=")[1])
			if err != nil {
				return 0, fmt.Errorf(
					"can't convert Leader value from '%s' to PID: '%s'",
					line,
					err,
				)
			}

			return pid, nil
		}
	}

	return 0, fmt.Errorf("PID info is not found in machinectl show '%s'", name)
}
