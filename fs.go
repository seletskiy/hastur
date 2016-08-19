package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/seletskiy/hierr"
)

func getFSType(root string) (string, error) {
	command := exec.Command("findmnt", "-o", "fstype", "-nfT", root)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", formatExecError(command, err, output)
	}

	return strings.TrimSpace(string(output)), nil
}

func createBaseDirForPackages(
	rootDir string,
	packages []string,
	storageEngine storage,
) (exists bool, dirName string, err error) {
	imageName := fmt.Sprintf(
		"%x",
		sha256.Sum224([]byte(strings.Join(packages, ","))),
	)

	imageDir := getImageDir(rootDir, imageName)
	if isExists(imageDir) && !isExists(imageDir, ".hastur") {
		err = storageEngine.DeInitImage(imageName)
		if err != nil {
			return false, "", hierr.Errorf(
				err, "can't deinitialize image %s", imageName,
			)
		}
	}

	if !isExists(imageDir) {
		err = storageEngine.InitImage(imageName)
		if err != nil {
			return false, "", hierr.Errorf(
				err, "can't initialize image %s", imageName,
			)
		}

		return false, imageName, nil
	} else {
		return true, imageName, nil
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

func getImageDir(rootDir string, imageName string) string {
	return filepath.Join(rootDir, "images", imageName)
}

func getBaseDirs(rootDir string) ([]string, error) {
	return filepath.Glob(filepath.Join(rootDir, "base.#*"))
}

func removeContainerDir(containerDir string) error {
	cmd := exec.Command("rm", "-rf", containerDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"can't remove dir %s: %s\n%s", containerDir, err, output,
		)
	}

	return nil
}

func isExists(path ...string) bool {
	_, err := os.Stat(filepath.Join(path...))
	return !os.IsNotExist(err)
}
