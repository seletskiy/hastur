package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func nspawn(
	storageEngine storage,
	rootDir, baseDir, containerName string, bridge string,
	networkAddress string,
	ephemeral bool,
	commandLine []string,
) error {
	containerDir := filepath.Join(rootDir, "containers", containerName)
	containerRoot := filepath.Join(containerDir, ".nspawn.root")

	err := storageEngine.Merge(
		baseDir, filepath.Join(containerDir, "root"), containerRoot,
	)

	if err != nil {
		return fmt.Errorf(
			"storage can't create rootfs for nspawn: %s", err,
		)
	}

	if err != nil {
		return fmt.Errorf(
			"can't setup overlayfs for '%s': %s", containerName, err,
		)
	}

	if ephemeral {
		defer removeContainerDir(containerDir)
	}

	defer umount(containerRoot)

	bootstrapper := "/.hastur.exec"
	err = installBootstrapExecutable(containerRoot, bootstrapper)
	if err != nil {
		return err
	}

	controlPipeName := bootstrapper + ".control"
	controlPipePath := filepath.Join(containerRoot, controlPipeName)

	err = syscall.Mknod(controlPipePath, syscall.S_IFIFO|0644, 0)
	if err != nil {
		return fmt.Errorf(
			"can't create control pipe for bootstrapper: %s", err,
		)
	}

	defer os.Remove(controlPipePath)

	args := []string{
		"-M", containerName + containerSuffix,
		"-n", "--network-bridge", bridge,
		"-D", containerRoot,
		bootstrapper,
		controlPipeName,
	}

	command := exec.Command(
		"systemd-nspawn",
		append(args, commandLine...)...,
	)

	command.Env = []string{}

	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err = command.Start()
	if err != nil {
		return err
	}

	_, err = ioutil.ReadFile(controlPipePath)
	if err != nil {
		return err
	}

	pid, err := getContainerLeaderPID(containerName)
	if err != nil {
		return err
	}

	err = mountNetworkNamespace(pid, containerName)
	if err != nil {
		return err
	}

	err = setupNetwork(containerName, networkAddress)
	if err != nil {
		return err
	}

	defer umountNetorkNamespace(containerName)

	err = ioutil.WriteFile(controlPipePath, []byte{}, 0)
	if err != nil {
		return fmt.Errorf("can't write to control pipe: %s", err)
	}

	return command.Wait()
}
