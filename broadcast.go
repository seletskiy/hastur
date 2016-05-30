package main

import "net"

func allFF(b []byte) bool {
	for _, c := range b {
		if c != 0xff {
			return false
		}
	}
	return true
}

func bytesEqual(x, y []byte) bool {
	if len(x) != len(y) {
		return false
	}
	for i, b := range x {
		if y[i] != b {
			return false
		}
	}
	return true
}

var v4InV6Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

func broadcast(ip net.IP, mask net.IPMask) net.IP {
	if len(mask) == net.IPv6len && len(ip) == net.IPv4len && allFF(mask[:12]) {
		mask = mask[12:]
	}
	if len(mask) == net.IPv4len && len(ip) == net.IPv6len && bytesEqual(ip[:12], v4InV6Prefix) {
		ip = ip[12:]
	}
	n := len(ip)
	if n != len(mask) {
		return nil
	}
	out := make(net.IP, n)
	for i := 0; i < n; i++ {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}
