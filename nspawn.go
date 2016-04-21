package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func nspawn(
	storageEngine storage,
	rootDir, baseDir, containerName string,
	hostNetwork bool,
	bridge string,
	networkAddress string,
	ephemeral bool, keepFailed bool,
	commandLine []string,
) (err error) {
	containerDir := getContainerDir(rootDir, containerName)

	containerPrivateRoot := getContainerPrivateRoot(rootDir, containerName)

	err = storageEngine.Merge(
		baseDir,
		getContainerDataRoot(rootDir, containerName),
		containerPrivateRoot,
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

	var execErr error

	if ephemeral {
		defer func() {
			if execErr != nil && keepFailed {
				return
			}

			removeErr := removeContainerDir(containerDir)
			if removeErr != nil {
				if err == nil {
					err = removeErr
					return
				}

				log.Println(
					"ERROR: can't remove container directory %s: %s",
					containerDir, err,
				)
			}
		}()
	}

	defer umount(containerPrivateRoot)

	bootstrapper := "/.hastur.exec"
	err = installBootstrapExecutable(containerPrivateRoot, bootstrapper)
	if err != nil {
		return err
	}

	controlPipeName := bootstrapper + ".control"
	controlPipePath := filepath.Join(containerPrivateRoot, controlPipeName)

	err = syscall.Mknod(controlPipePath, syscall.S_IFIFO|0644, 0)
	if err != nil {
		return fmt.Errorf(
			"can't create control pipe for bootstrapper: %s", err,
		)
	}

	defer os.Remove(controlPipePath)

	args := []string{
		"-M", containerName + containerSuffix,
		"-D", containerPrivateRoot,
	}

	if !hostNetwork {
		args = append(args, "-n", "--network-bridge", bridge)
	}

	args = append(args, bootstrapper, controlPipeName)

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

	if !hostNetwork {
		err = mountNetworkNamespace(pid, containerName)
		if err != nil {
			return err
		}

		err = setupNetwork(containerName, networkAddress)
		if err != nil {
			return err
		}

		defer umountNetorkNamespace(containerName)
	}

	err = ioutil.WriteFile(controlPipePath, []byte{}, 0)
	if err != nil {
		return fmt.Errorf("can't write to control pipe: %s", err)
	}

	execErr = command.Wait()
	return execErr
}
