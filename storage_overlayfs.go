package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultOverlayFSSize = "1G"

type overlayFSStorage struct {
	tmpfsSize string
	rootDir   string
}

func NewOverlayFSStorage(rootDir, spec string) (storage, error) {
	args := strings.Split(spec, ":")
	size := defaultOverlayFSSize

	// TODO: validate size parameter
	if len(args) == 2 {
		size = args[1]
	}

	return &overlayFSStorage{
		rootDir:   rootDir,
		tmpfsSize: size,
	}, nil
}

func (storage *overlayFSStorage) Init() error {
	FSType, err := getFSType(storage.rootDir)
	if err != nil {
		return fmt.Errorf(
			"can't get FS type for '%s': %s", storage.rootDir, err,
		)
	}

	switch FSType {
	case "tmpfs", "ext", "ext2", "ext3", "ext4", "btrfs":
		return nil

	default:
		fmt.Printf("WARNING! %s is not currently supported.\n", FSType)
		fmt.Println("         overlayfs over tmpfs will be used and")
		fmt.Println("         containers will not persist across reboots.")
		fmt.Println()

		err := storage.fixUnsupportedFS()
		if err != nil {
			return err
		}
	}

	return nil
}

func (storage *overlayFSStorage) Merge(base, data, target string) error {
	return mountOverlay(
		base,
		data,
		filepath.Join(filepath.Dir(data), ".overlay.workdir"),
		target,
	)
}

func (storage *overlayFSStorage) Destroy() error {
	cmd := exec.Command("rm", "-rf", storage.rootDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("can't remove root: %s\n%s", err, output)
	}

	return umount(storage.rootDir)
}

func (storage *overlayFSStorage) fixUnsupportedFS() error {
	tmpfsMounted, err := isMounted("tmpfs", storage.rootDir)
	if err != nil {
		return fmt.Errorf(
			"can't check is tmpfs mounted on '%s': %s", storage.rootDir, err,
		)
	}

	if !tmpfsMounted {
		err := mountTmpfs(storage.rootDir, storage.tmpfsSize)
		if err != nil {
			return fmt.Errorf(
				"can't mount tmpfs of size %s on '%s': %s",
				storage.tmpfsSize, storage.rootDir, err,
			)
		}
	}

	return nil
}
