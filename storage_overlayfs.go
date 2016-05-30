package main

import (
	"fmt"
	"os"
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

func (storage *overlayFSStorage) InitImage(image string) error {
	return os.MkdirAll(getImageDir(storage.rootDir, image), 0755)
}

func (storage *overlayFSStorage) DeInit() error {
	return nil
}

func (storage *overlayFSStorage) InitContainer(
	baseDir string,
	containerName string,
) error {
	containerDir := getContainerDir(storage.rootDir, containerName)

	containerRoot := storage.GetContainerRoot(containerName)

	for _, dir := range []string{"root", ".nspawn.root", ".overlay.workdir"} {
		err := os.MkdirAll(
			filepath.Join(containerDir, dir),
			0755,
		)

		if err != nil {
			return err
		}
	}

	err := mountOverlay(
		getImageDir(storage.rootDir, baseDir),
		filepath.Join(containerDir, "root"),
		filepath.Join(containerDir, ".overlay.workdir"),
		containerRoot,
	)

	if err != nil {
		return fmt.Errorf(
			"can't mount overlay fs [%s] for '%s': %s",
			baseDir, containerName, err,
		)
	}

	return nil
}

func (storage *overlayFSStorage) GetContainerRoot(containerName string) string {
	containerDir := getContainerDir(storage.rootDir, containerName)

	return filepath.Join(containerDir, ".nspawn.root")
}

func (storage *overlayFSStorage) DeInitContainer(containerName string) error {
	return umount(storage.GetContainerRoot(containerName))
}

func (storage *overlayFSStorage) Destroy() error {
	return umountRecursively(storage.rootDir)
}

func (storage *overlayFSStorage) DestroyContainer(containerName string) error {
	_ = storage.DeInitContainer(containerName)

	return removeContainerDir(getContainerDir(storage.rootDir, containerName))
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
