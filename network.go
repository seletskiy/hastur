package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/reconquest/executil-go"
	"github.com/reconquest/ser-go"
)

func ensureBridge(bridge string) error {
	command := exec.Command("brctl", "addbr", bridge)
	_, stderr, err := executil.Run(command)
	if err != nil {
		prefix := fmt.Sprintf("device %s already exists;", bridge)
		if strings.HasPrefix(string(stderr), prefix) {
			return nil
		}

		return err
	}

	return nil
}

func ensureBridgeInterfaceUp(bridge string) error {
	command := exec.Command("ip", "link", "set", "dev", bridge, "up")
	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func ensureIPv4Forwarding() error {
	fileIpForward := "/proc/sys/net/ipv4/ip_forward"
	valueIpForward, err := ioutil.ReadFile(fileIpForward)
	if err != nil {
		return ser.Errorf(
			err, "can't read file %s", fileIpForward,
		)
	}

	if strings.Contains(string(valueIpForward), "0") {
		err = ioutil.WriteFile(
			fileIpForward, []byte("1\n"),
			os.FileMode(0644),
		)
		if err != nil {
			return ser.Errorf(
				err, "can't write '1' to file %s", fileIpForward,
			)
		}
	}

	return nil
}

func copyInterfaceRoutesToBridge(iface, bridge string) error {
	command := exec.Command("ip", "route", "show", "dev", iface)
	output, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	rawIPOutput := strings.Split(string(output), "\n")
	for _, line := range rawIPOutput {
		if line == "" {
			continue
		}

		trimmedLine := strings.TrimSpace(line)
		trimmedLine = strings.Replace(trimmedLine, "  ", " ", -1)

		err = execIpRoute(
			"delete", iface,
			strings.Split(trimmedLine, " ")...,
		)
		if err != nil {
			return err
		}

		err = execIpRoute(
			"add", bridge,
			strings.Split(trimmedLine, " ")...,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func execIpRoute(action string, iface string, args ...string) error {
	command := exec.Command(
		"ip", append(
			[]string{"route", action, "dev", iface},
			args...,
		)...,
	)

	_, stderr, err := executil.Run(command)
	if err != nil {
		if bytes.HasPrefix(
			stderr,
			[]byte("RTNETLINK answers: File exists"),
		) {
			return nil
		}

		return err
	}

	return nil
}

func copyInterfaceAddressToBridge(iface string, bridge string) error {
	addrs, err := getHostIPs(iface)
	if err != nil {
		return ser.Errorf(
			err, "can't get host ip addresses for interface %s", iface,
		)
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return ser.Errorf(
				err, "can't parse net address '%s'", addr.String(),
			)
		}
		if ip.To4() == nil {
			continue
		}

		broadcast := broadcast(ip, ip.DefaultMask())

		command := exec.Command(
			"ip", "addr", "add",
			"dev", bridge, addr.String(),
			"broadcast", broadcast.String(),
		)
		_, stderr, err := executil.Run(command)
		if err != nil {
			if bytes.HasPrefix(
				stderr,
				[]byte("RTNETLINK answers: File exists"),
			) {
				return nil
			}

			return err
		}
	}

	return nil
}

func addInterfaceToBridge(iface, bridge string) error {
	command := exec.Command("brctl", "addif", bridge, iface)
	_, stderr, err := executil.Run(command)
	if err != nil {
		prefix := fmt.Sprintf("device %s is already a member", iface)
		if strings.HasPrefix(string(stderr), prefix) {
			return nil
		}

		return err
	}

	return nil
}

func getContainerIP(containerName string) (string, error) {
	command := exec.Command("ip", "-n", containerName, "addr", "show", "host0")
	output, _, err := executil.Run(command)
	if err != nil {
		return "", err
	}

	rawIPOutput := strings.Split(string(output), "\n")
	for _, line := range rawIPOutput {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "inet ") {
			inet := strings.Fields(trimmedLine)
			if len(inet) < 2 {
				return "", fmt.Errorf(
					"invalid output from ip: %q", line,
				)
			}

			return inet[1], nil
		}
	}

	return "", nil
}

func setupNetwork(namespace string, address string, gateway string) error {
	err := ensureAddress(namespace, address, "host0")
	if err != nil {
		return err
	}

	err = upInterface(namespace, "host0")
	if err != nil {
		return err
	}

	gatewayIP, _, err := net.ParseCIDR(gateway)
	if err != nil {
		return err
	}

	err = addDefaultRoute(namespace, "host0", gatewayIP.String())
	if err != nil {
		return err
	}

	return nil
}

func addDefaultRoute(namespace string, dev string, gateway string) error {
	args := []string{"route", "add", "default", "via", gateway}
	if namespace != "" {
		args = append([]string{"-n", namespace}, args...)
	}

	command := exec.Command("ip", args...)

	_, stderr, err := executil.Run(command)
	if err != nil {
		if bytes.HasPrefix(stderr, []byte("RTNETLINK answers: File exists")) {
			return nil
		}

		return err
	}

	return nil
}

func ensureAddress(namespace string, address string, dev string) error {
	args := []string{"addr", "add", address, "dev", dev}
	if namespace != "" {
		args = append([]string{"-n", namespace}, args...)
	}

	command := exec.Command("ip", args...)

	_, stderr, err := executil.Run(command)
	if err != nil {
		if bytes.HasPrefix(stderr, []byte("RTNETLINK answers: File exists")) {
			return nil
		}

		return err
	}

	return nil
}

func cleanupNetworkInterface(name string) error {
	interfaceName := "vb-" + name
	if len(interfaceName) > 14 {
		interfaceName = interfaceName[:14] // seems like it get cutted by 14 chars
	}

	args := []string{"link", "delete", interfaceName}

	command := exec.Command("ip", args...)

	_, stderr, err := executil.Run(command)
	if err != nil {
		if bytes.HasPrefix(stderr, []byte("Cannot find device")) {
			return nil
		}

		return err
	}

	return nil
}

func setupBridge(dev string, address string) error {
	return ensureAddress("", address, dev)
}

func upInterface(namespace string, dev string) error {
	command := exec.Command(
		"ip", "-n", namespace, "link", "set", "up", dev,
	)

	_, _, err := executil.Run(command)
	if err != nil {
		return err
	}

	return nil
}

func generateRandomNetwork(address *net.IPNet) string {
	tick := float64(time.Now().UnixNano() / 1000000)

	ones, bits := address.Mask.Size()
	zeros := bits - ones
	uniqIPsAmount := math.Pow(2.0, float64(zeros))

	rawIP := math.Mod(tick, uniqIPsAmount)

	remainder := rawIP

	remainder, octet4 := math.Modf(remainder / 255.0)
	remainder, octet3 := math.Modf(remainder / 255.0)
	remainder, octet2 := math.Modf(remainder / 255.0)

	base := address.IP

	address.IP = net.IPv4(
		byte(remainder)|base[0],
		byte(octet2*255)|base[1],
		byte(octet3*255)|base[2],
		byte(octet4*255)|base[3],
	)

	address.IP.Mask(address.Mask)

	return address.String()
}

func getHostIPs(interfaceName string) ([]net.Addr, error) {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var iface net.Interface
	for _, hostInterface := range hostInterfaces {
		if hostInterface.Name == interfaceName {
			iface = hostInterface
			break
		}
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, errors.New("no ip addresses assigned to interface")
	}

	return addrs, nil
}
