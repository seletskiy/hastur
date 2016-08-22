package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/kovetskiy/executil"
	"github.com/reconquest/ser-go"
)

func nspawn(
	storageEngine storage,
	containerName string,
	bridge string,
	networkAddress string, bridgeAddress string,
	ephemeral bool, keepFailed bool, quiet bool,
	commandLine []string,
) (err error) {
	defer storageEngine.DeInitContainer(containerName)

	if err != nil {
		return ser.Errorf(
			err,
			"storage can't create rootfs for nspawn",
		)
	}

	if err != nil {
		return ser.Errorf(
			err,
			"can't setup overlayfs for '%s'", containerName,
		)
	}

	if ephemeral {
		defer func() {
			if err != nil && keepFailed {
				return
			}

			removeErr := storageEngine.DestroyContainer(containerName)
			if removeErr != nil {
				err = removeErr

				fmt.Fprintln(
					os.Stderr,
					ser.Errorf(
						err, "can't remove container '%s'", containerName,
					).HierarchicalError(),
				)
			}
		}()
	}

	bootstrapper := "/.hastur.exec"
	err = installBootstrapExecutable(
		storageEngine.GetContainerRoot(containerName),
		bootstrapper,
	)
	if err != nil {
		return err
	}

	controlPipeName := bootstrapper + ".control"
	controlPipePath := filepath.Join(
		storageEngine.GetContainerRoot(containerName),
		controlPipeName,
	)

	err = syscall.Mknod(controlPipePath, syscall.S_IFIFO|0644, 0)
	if err != nil {
		return ser.Errorf(
			err,
			"can't create control pipe for bootstrapper",
		)
	}

	defer os.Remove(controlPipePath)

	// we ignore error there because interface may not exist
	_ = umountNetorkNamespace(containerName)
	_ = cleanupNetworkInterface(containerName)

	defer cleanupNetworkInterface(containerName)

	err = addPostroutingMasquarading(bridge)
	if err != nil {
		return ser.Errorf(
			err,
			"can't add masquarading rules on the '%s'",
			bridge,
		)
	}

	defer removePostroutingMasquarading(bridge)

	command := exec.Command(
		"systemd-machine-id-setup",
		"--root", storageEngine.GetContainerRoot(containerName),
	)
	_, _, err = executil.Run(command)
	if err != nil {
		return err
	}

	args := []string{
		"-M", containerName + containerSuffix,
		"-D", storageEngine.GetContainerRoot(containerName),
	}

	args = append(args, "-n", "--network-bridge", bridge)

	if quiet {
		args = append(args, "-q")
	}

	args = append(args, bootstrapper, controlPipeName)

	command = exec.Command(
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

	defer command.Process.Kill()

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

	defer umountNetorkNamespace(containerName)

	err = setupNetwork(containerName, networkAddress, bridgeAddress)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(controlPipePath, []byte{}, 0)
	if err != nil {
		return ser.Errorf(
			err, "can't write to control pipe")
	}

	err = command.Wait()
	return err
}
