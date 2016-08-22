package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/reconquest/ser-go"
)

const (
	containerSuffix = `.hastur`
	defaultPackages = `bash,coreutils,iproute2,iputils,libidn,nettle`
	version         = `3.0`
	usage           = `hastur the unspeakable - zero-conf systemd container manager.

hastur is a simple wrapper around systemd-nspawn, that will start container
with overlayfs, pre-installed packages and bridged network available out
of the box.

hastur operates over specified root directory, which will hold base FS
for containers and numerous amount of containers derived from base FS.

Primary use of hastur is testing purposes, running testcases for distributed
services and running local network of trusted containers.

Usage:
    hastur -h | --help
    hastur [options] [-b=] [-s=] [-a=] [-p <packages>...] [-n=] -S [--] [<command>...]
    hastur [options] [-s=] -Q [-j] [<name>...]
    hastur [options] [-s=] -D [-f] <name>
    hastur [options] [-s=] --free

Options:
    -h --help        Show this help.
    -r <root>        Root directory which will hold containers.
                      [default: /var/lib/hastur/]
    -q               Be quiet. Do not report status messages from nspawn.
    -f               Force operation.
    -s <storage>     Use specified storageSpec backend for container base
                      images and containers themselves. By default, overlayfs
                      will be used to provide COW for base system. If overlayfs
                      is not possible on current FS and no other storageSpec
                      engine is possible, tmpfs will be mounted in specified
                      root dir to provide groundwork for overlayfs.
                      [default: autodetect]
       <storage>     Possible values are:
                      * autodetect - use one of available storage engines
                      depending on current FS.
                      * overlayfs:N - use current FS and overlayfs on top;
                      if overlayfs is unsupported on current FS, mount tmpfs of
                      size N first.
                      * zfs:POOL - use ZFS and use <root> located on POOL.

Create options:
    -S               Create and start container.
       <command>     Execute specified command in created
                      container.
      -b <bridge>    Bridge interface name and, optionally, an address,
                      separated by colon.
                      If bridge does not exists, it will be automatically
                      created.
                      [default: br0:10.0.0.1/8]
      -t <iface>     Use host network and gain access to external network.
                      Interface will pair given interface with bridge.
      -p <packages>  Packages to install, separated by comma.
                      [default: ` + defaultPackages + `]
      -n <name>      Use specified container name. If not specified, randomly
                      generated name will be used and container will be
                      considered ephemeral, e.g. will be destroyed on <command>
                      exit.
      -a <address>   Use specified IP address/netmask. If not specified,
                      automatically generated adress from 10.0.0.0/8 will
                      be used.
      -k             Keep container after exit if it name was autogenerated.
      -x <dir>       Copy entries of specified directory into created
                      container root directory.
      -e             Keep container after exit if executed <command> failed.

Query options:
    -Q               Show information about containers in the <root> dir.
       <name>        Query container's options.
    -j               Output information using JSON format.
Destroy options:
    -D               Destroy specified container.
    --free           Completely remove all data in <root> directory with
                      containers and base images.
`
)

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if os.Args[0] == "/.hastur.exec" && len(os.Args) >= 2 {
		err := execBootstrap()
		if err != nil {
			fatal(err)
		}
	}

	args, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		panic(err)
	}

	var (
		rootDir     = args["-r"].(string)
		storageSpec = args["-s"].(string)
	)

	storageEngine, err := createStorageFromSpec(rootDir, storageSpec)
	if err != nil {
		fatal(ser.Errorf(err, "can't initialize storage"))
	}

	switch {
	case args["-S"].(bool):
		err = createAndStart(args, storageEngine)
	case args["-Q"].(bool):
		err = queryContainers(args, storageEngine)
	case args["-D"].(bool):
		err = destroyContainer(args, storageEngine)
	case args["--free"].(bool):
		err = destroyRoot(args, storageEngine)
	}

	if err != nil {
		fatal(err)
	}
}

func execBootstrap() error {
	command := []string{}

	if len(os.Args) == 2 {
		command = []string{"/bin/bash"}
	} else {
		if len(os.Args) == 3 && strings.Contains(os.Args[2], " ") {
			command = []string{"/bin/bash", "-c", os.Args[2]}
		} else {
			command = os.Args[2:]
		}
	}

	ioutil.WriteFile(os.Args[1], []byte{}, 0)
	ioutil.ReadFile(os.Args[1])

	err := os.Remove(os.Args[1])
	if err != nil {
		return ser.Errorf(
			err,
			"can't remove control file '%s'", os.Args[1],
		)
	}

	err = syscall.Exec(command[0], command[0:], os.Environ())
	if err != nil {
		return ser.Errorf(
			err,
			"can't execute command %q", os.Args[2:],
		)
	}

	return nil
}

func destroyContainer(
	args map[string]interface{},
	storageEngine storage,
) error {
	var (
		containerName = args["<name>"].(string)
	)

	err := storageEngine.DestroyContainer(containerName)

	_ = umountNetorkNamespace(containerName)

	err = cleanupNetworkInterface(containerName)
	if err != nil {
		log.Println(err)
	}

	return err
}

func showBaseDirsInfo(
	args map[string]interface{},
	storageEngine storage,
) error {
	var (
		rootDir = args["-r"].(string)
	)

	baseDirs, err := getBaseDirs(rootDir)
	if err != nil {
		return ser.Errorf(
			err,
			"can't get base dirs from '%s'", rootDir,
		)
	}

	for _, baseDir := range baseDirs {
		packages, err := listExplicitlyInstalled(baseDir)
		if err != nil {
			return ser.Errorf(
				err,
				"can't list explicitly installed packages in '%s'",
				baseDir,
			)
		}

		fmt.Println(baseDir)
		for _, packageName := range packages {
			fmt.Printf("\t%s\n", packageName)
		}
	}

	return nil
}

func createAndStart(
	args map[string]interface{},
	storageEngine storage,
) error {
	var (
		bridgeInfo        = args["-b"].(string)
		rootDir           = args["-r"].(string)
		packagesList      = args["-p"].([]string)
		containerName, _  = args["-n"].(string)
		commandLine       = args["<command>"].([]string)
		networkAddress, _ = args["-a"].(string)
		force             = args["-f"].(bool)
		keep              = args["-k"].(bool)
		keepFailed        = args["-e"].(bool)
		copyingDir, _     = args["-x"].(string)
		hostInterface, _  = args["-t"].(string)
		quiet             = args["-q"].(bool)
	)

	err := ensureIPv4Forwarding()
	if err != nil {
		return ser.Errorf(
			err,
			"can't enable ipv4 forwarding",
		)
	}

	bridgeDevice, bridgeAddress := parseBridgeInfo(bridgeInfo)
	err = ensureBridge(bridgeDevice)
	if err != nil {
		return ser.Errorf(
			err,
			"can't create bridge interface '%s'", bridgeDevice,
		)
	}

	err = ensureBridgeInterfaceUp(bridgeDevice)
	if err != nil {
		return ser.Errorf(
			err,
			"can't set bridge '%s' up",
			bridgeDevice,
		)
	}

	if bridgeAddress != "" {
		err = setupBridge(bridgeDevice, bridgeAddress)
		if err != nil {
			return ser.Errorf(
				err,
				"can't assign address '%s' on bridge '%s'",
				bridgeAddress,
				bridgeDevice,
			)
		}
	}

	if hostInterface != "" {
		err := addInterfaceToBridge(hostInterface, bridgeDevice)
		if err != nil {
			return ser.Errorf(
				err,
				"can't bind host's ethernet '%s' to '%s'",
				hostInterface,
				bridgeDevice,
			)
		}

		err = copyInterfaceAddressToBridge(hostInterface, bridgeDevice)
		if err != nil {
			return ser.Errorf(
				err,
				"can't copy address from host's '%s' to '%s'",
				hostInterface,
				bridgeDevice,
			)
		}

		err = copyInterfaceRoutesToBridge(hostInterface, bridgeDevice)
		if err != nil {
			return ser.Errorf(
				err,
				"can't copy routes from host's '%s' to '%s'",
				hostInterface,
				bridgeDevice,
			)
		}
	}

	ephemeral := false
	if containerName == "" {
		generatedName := generateContainerName()
		if !keep {
			ephemeral = true

			if !keepFailed && !quiet {
				fmt.Println(
					"Container is ephemeral and will be deleted after exit.",
				)
			}
		}

		containerName = generatedName

		fmt.Printf("Container name: %s\n", containerName)
	}

	allPackages := []string{}
	for _, packagesGroup := range packagesList {
		packages := strings.Split(packagesGroup, ",")
		allPackages = append(allPackages, packages...)
	}

	cacheExists, baseDir, err := createBaseDirForPackages(
		rootDir,
		allPackages,
		storageEngine,
	)
	if err != nil {
		return ser.Errorf(
			err,
			"can't create base dir '%s'", baseDir,
		)
	}

	if !cacheExists || force {
		fmt.Println("Installing packages")
		err = installPackages(getImageDir(rootDir, baseDir), allPackages)
		if err != nil {
			return ser.Errorf(
				err,
				"can't install packages into '%s'", rootDir,
			)
		}

		err = ioutil.WriteFile(
			filepath.Join(getImageDir(rootDir, baseDir), ".hastur"),
			nil, 0644,
		)
		if err != nil {
			return ser.Errorf(
				err, "can't create .hastur file in image directory",
			)
		}
	}

	err = storageEngine.InitContainer(baseDir, containerName)
	if err != nil {
		return ser.Errorf(
			err,
			"can't create directory layout under '%s'", rootDir,
		)
	}

	if networkAddress == "" {
		_, baseIPNet, _ := net.ParseCIDR("10.0.0.0/8")
		networkAddress = generateRandomNetwork(baseIPNet)

		if !quiet {
			fmt.Printf("Container will use IP: %s\n", networkAddress)
		}
	}

	if copyingDir != "" {
		err = copyDir(copyingDir, getImageDir(rootDir, baseDir))
		if err != nil {
			return ser.Errorf(
				err,
				"can't copy %s to container root", copyingDir,
			)
		}
	}

	err = nspawn(
		storageEngine,
		containerName,
		bridgeDevice, networkAddress, bridgeAddress,
		ephemeral, keepFailed, quiet,
		commandLine,
	)

	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			os.Exit(err.Sys().(syscall.WaitStatus).ExitStatus())
		}

		return ser.Errorf(err, "command execution failed")
	}

	return nil
}

func destroyRoot(
	args map[string]interface{},
	storageEngine storage,
) error {
	err := storageEngine.Destroy()
	if err != nil {
		return ser.Errorf(
			err, "can't destroy storage",
		)
	}

	return nil
}

func generateContainerName() string {
	tuples := []string{"ir", "oh", "at", "op", "un", "ed"}
	triples := []string{"gep", "vin", "kut", "lop", "man", "zod"}
	all := append(append([]string{}, tuples...), triples...)

	getTuple := func() string {
		return tuples[rand.Intn(len(tuples))]
	}

	getTriple := func() string {
		return triples[rand.Intn(len(triples))]
	}

	getAny := func() string {
		return all[rand.Intn(len(all))]
	}

	id := []string{
		getTuple(),
		getTriple(),
		"-",
		getTuple(),
		getTriple(),
		getTuple(),
		"-",
		getAny(),
	}

	return strings.Join(id, "")
}

func parseBridgeInfo(bridgeInfo string) (dev, address string) {
	parts := strings.Split(bridgeInfo, ":")
	if len(parts) == 1 {
		return parts[0], ""
	} else {
		return parts[0], parts[1]
	}
}

func createStorageFromSpec(rootDir, storageSpec string) (storage, error) {
	var storageEngine storage
	var err error

	switch {
	case storageSpec == "autodetect":
		storageSpec = "overlayfs"
		fallthrough

	case strings.HasPrefix(storageSpec, "overlayfs"):
		storageEngine, err = NewOverlayFSStorage(rootDir, storageSpec)

	case strings.HasPrefix(storageSpec, "zfs"):
		storageEngine, err = NewZFSStorage(rootDir, storageSpec)
	}

	if err != nil {
		return nil, ser.Errorf(
			err, "can't create storage '%s'", storageSpec,
		)
	}

	err = storageEngine.Init()
	if err != nil {
		return nil, ser.Errorf(
			err, "can't init storage '%s'", storageSpec,
		)
	}

	return storageEngine, nil
}
