package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/reconquest/executil-go"
	"github.com/reconquest/ser-go"
)

func isMounted(device, mountpoint string) (bool, error) {
	absPath, err := filepath.Abs(mountpoint)
	if err != nil {
		return false, err
	}

	command := exec.Command("findmnt", device, absPath)
	_, _, err = executil.Run(command)
	if err != nil {
		if executil.IsExitError(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func mountTmpfs(target string, size string) error {
	command := exec.Command(
		"mount", "-t", "tmpfs", "-o", "size="+size, "tmpfs", target,
	)

	_, _, err := executil.Run(command)
	if err != nil {
		return err
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

	_, _, err = executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func mountNetworkNamespace(PID int, target string) error {
	netnsDir := "/var/run/netns"
	if _, err := os.Stat(netnsDir); os.IsNotExist(err) {
		err := os.Mkdir(netnsDir, 0755)
		if err != nil {
			return ser.Errorf(
				err,
				"can't create dir '%s'", netnsDir,
			)
		}
	}

	bindTarget := filepath.Join(netnsDir, target)

	err := ioutil.WriteFile(bindTarget, []byte{}, 0644)
	if err != nil {
		return ser.Errorf(
			err, "can't touch file '%s'", bindTarget,
		)
	}

	return mountBind(
		filepath.Join("/proc", fmt.Sprint(PID), "ns/net"), bindTarget,
	)
}

func mountBind(source, target string) error {
	command := exec.Command("mount", "--bind", source, target)

	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func umountNetorkNamespace(name string) error {
	bindTarget := filepath.Join("/var/run/netns", name)

	err := umount(bindTarget)
	if err != nil {
		return ser.Errorf(
			err, "can't umount %s", bindTarget,
		)
	}

	err = os.Remove(bindTarget)
	if err != nil {
		return ser.Errorf(
			err, "can't remove %s", bindTarget,
		)
	}

	return nil
}

func umountRecursively(target string) error {
	command := exec.Command("umount", "-R", target)
	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func umount(target string) error {
	command := exec.Command("umount", target)
	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}
