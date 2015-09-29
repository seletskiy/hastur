package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func isMounted(device, mountpoint string) (bool, error) {
	absPath, err := filepath.Abs(mountpoint)
	if err != nil {
		return false, err
	}

	command := exec.Command("findmnt", device, absPath)
	output, err := command.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}

		return false, formatExecError(command, err, output)
	}

	return true, nil
}

func mountTmpfs(target string, size string) error {
	command := exec.Command(
		"mount", "-t", "tmpfs", "-o", "size="+size, "tmpfs", target,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func mountOverlay(lower, upper, work, target string) error {
	lowerAbsPath, err := filepath.Abs(lower)
	if err != nil {
		return formatAbsPathError(lower, err)
	}

	upperAbsPath, err := filepath.Abs(upper)
	if err != nil {
		return formatAbsPathError(upper, err)
	}

	workAbsPath, err := filepath.Abs(work)
	if err != nil {
		return formatAbsPathError(work, err)
	}

	command := exec.Command(
		"mount", "-t", "overlay", "-o",
		strings.Join([]string{
			"lowerdir=" + lowerAbsPath,
			"upperdir=" + upperAbsPath,
			"workdir=" + workAbsPath,
		}, ","),
		"overlay", target,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func mountNetworkNamespace(PID int, target string) error {
	netnsDir := "/var/run/netns"
	if _, err := os.Stat(netnsDir); os.IsNotExist(err) {
		err := os.Mkdir(netnsDir, 0755)
		if err != nil {
			return fmt.Errorf(
				"can't create dir '%s': %s", netnsDir, err,
			)
		}
	}

	bindTarget := filepath.Join(netnsDir, target)

	err := ioutil.WriteFile(bindTarget, []byte{}, 0644)
	if err != nil {
		return fmt.Errorf("can't touch file '%s': %s", bindTarget, err)
	}

	return mountBind(
		filepath.Join("/proc", fmt.Sprint(PID), "ns/net"), bindTarget,
	)
}

func mountBind(source, target string) error {
	command := exec.Command("mount", "--bind", source, target)

	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func umountNetorkNamespace(name string) error {
	bindTarget := filepath.Join("/var/run/netns", name)

	err := umount(bindTarget)
	if err != nil {
		return err
	}

	return os.Remove(bindTarget)
}

func umount(target string) error {
	command := exec.Command("umount", target)
	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}
