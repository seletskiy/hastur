package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getFSType(root string) (string, error) {
	command := exec.Command("findmnt", "-o", "fstype", "-nfT", root)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", formatExecError(command, err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func createLayout(root string, containerName string) error {
	err := os.MkdirAll(filepath.Join(root, "containers"), 0755)
	if err != nil {
		return err
	}

	for _, dir := range []string{"root", ".nspawn.root", ".overlay.workdir"} {
		err = os.MkdirAll(
			filepath.Join(root, "containers", containerName, dir),
			0755,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func createBaseDirForPackages(
	root string, packages []string,
) (exists bool, dirName string, err error) {
	baseDir := fmt.Sprintf(
		"%s.#%x",
		filepath.Join(root, "base"),
		sha256.Sum224([]byte(strings.Join(packages, ","))),
	)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		err := os.Mkdir(baseDir, 0755)
		if err != nil {
			return false, "", fmt.Errorf(
				"can't create base dir '%s': %s",
				baseDir,
				err,
			)
		}

		return false, baseDir, nil
	} else {
		return true, baseDir, nil
	}
}

func installBootstrapExecutable(root string, target string) error {
	path, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return err
	}

	command := exec.Command("cp", path, filepath.Join(root, target))

	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func listContainers(rootDir string) ([]string, error) {
	containers := []string{}

	filepath.Walk(
		rootDir,
		func(path string, info os.FileInfo, err error) error {
			if path == rootDir {
				return nil
			}

			if info.IsDir() {
				containers = append(containers, filepath.Base(path))
				return filepath.SkipDir
			}

			return nil
		},
	)

	return containers, nil
}

func getContainerDir(rootDir string, containerName string) string {
	return filepath.Join(rootDir, "containers", containerName)
}

func getContainerPrivateRoot(rootDir string, containerName string) string {
	return filepath.Join(
		getContainerDir(rootDir, containerName),
		".nspawn.root",
	)
}

func getContainerDataRoot(rootDir string, containerName string) string {
	return filepath.Join(
		getContainerDir(rootDir, containerName),
		"root",
	)
}

func getBaseDirs(rootDir string) ([]string, error) {
	return filepath.Glob(filepath.Join(rootDir, "base.#*"))
}

func ensureRootDir(root string) error {
	return os.MkdirAll(root, 0755)
}

func removeContainer(rootDir, containerName string) error {
	containerDir := filepath.Join(rootDir, "containers", containerName)

	return removeContainerDir(containerDir)
}

func removeContainerDir(containerDir string) error {
	return os.RemoveAll(containerDir)
}
