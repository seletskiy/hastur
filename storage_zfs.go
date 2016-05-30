package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type zfsStorage struct {
	pool    string
	rootDir string
}

func doZFSCommand(parameters ...string) error {
	command := exec.Command("zfs", parameters...)
	output, err := command.CombinedOutput()
	if err != nil {
		return formatExecError(command, err, output)
	}

	return nil
}

func NewZFSStorage(rootDir, spec string) (storage, error) {
	args := strings.Split(spec, ":")
	pool := ""

	// TODO: validate pool parameter
	if len(args) == 2 {
		pool = args[1]
	}

	if pool == "" {
		return nil, fmt.Errorf(
			`pool name should be specified`,
		)
	}

	return &zfsStorage{
		pool:    pool,
		rootDir: rootDir,
	}, nil
}

func (storage *zfsStorage) Init() error {
	err := doZFSCommand(
		"create",
		"-p",
		filepath.Join(storage.pool, getContainerDir(storage.rootDir, "")),
	)
	if err != nil {
		return err
	}

	err = doZFSCommand(
		"create",
		"-p",
		filepath.Join(storage.pool, getImageDir(storage.rootDir, "")),
	)
	if err != nil {
		return err
	}

	return nil
}

func (storage *zfsStorage) InitImage(image string) error {
	err := doZFSCommand(
		"create",
		"-p",
		filepath.Join(storage.pool, getImageDir(storage.rootDir, image)),
	)
	if err != nil {
		return err
	}

	return nil
}

func (storage *zfsStorage) DeInit() error {
	return nil
}

func (storage *zfsStorage) InitContainer(
	baseDir string,
	containerName string,
) error {
	err := doZFSCommand(
		"snapshot",
		filepath.Join(
			storage.pool,
			getImageDir(storage.rootDir, baseDir),
		)+"@"+containerName,
	)

	if err != nil {
		return err
	}

	err = doZFSCommand(
		"clone",
		filepath.Join(
			storage.pool,
			getImageDir(storage.rootDir, baseDir),
		)+"@"+containerName,
		filepath.Join(
			storage.pool,
			getContainerDir(storage.rootDir, containerName),
		),
	)

	if err != nil {
		return err
	}

	return nil
}

func (storage *zfsStorage) GetContainerRoot(containerName string) string {
	containerDir := getContainerDir(storage.rootDir, containerName)

	return containerDir
}

func (storage *zfsStorage) DeInitContainer(containerName string) error {
	return nil
}

func (storage *zfsStorage) Destroy() error {
	return doZFSCommand(
		"destroy",
		"-r",
		filepath.Join(storage.pool, storage.rootDir),
	)
}

func (storage *zfsStorage) DestroyContainer(containerName string) error {
	return doZFSCommand(
		"destroy",
		filepath.Join(storage.pool, getContainerDir(
			storage.rootDir,
			containerName,
		)),
	)
}
