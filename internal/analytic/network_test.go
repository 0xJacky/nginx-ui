package analytic

import (
	stdnet "net"
	"testing"
)

func mustCIDR(t *testing.T, value string) stdnet.Addr {
	t.Helper()

	ip, ipNet, err := stdnet.ParseCIDR(value)
	if err != nil {
		t.Fatalf("failed to parse CIDR %q: %v", value, err)
	}
	ipNet.IP = ip
	return ipNet
}

func TestShouldCountNetworkInterfaceAcceptsWindowsEthernetPrivateIPv4(t *testing.T) {
	iface := networkInterfaceInfo{
		Name:         "Ethernet0",
		Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
		HardwareAddr: stdnet.HardwareAddr{0x00, 0x50, 0x56, 0xba, 0x25, 0x01},
		Addrs:        []stdnet.Addr{mustCIDR(t, "10.100.1.10/28")},
	}

	if !shouldCountNetworkInterface(iface) {
		t.Fatalf("expected Windows Ethernet interface with private IPv4 to be counted")
	}
}

func TestShouldCountNetworkInterfaceRejectsTapAdapter(t *testing.T) {
	iface := networkInterfaceInfo{
		Name:         "TAP-Windows Adapter V9",
		Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
		HardwareAddr: stdnet.HardwareAddr{0x00, 0xff, 0x55, 0x61, 0x3a, 0xd2},
		Addrs:        []stdnet.Addr{mustCIDR(t, "10.8.0.2/24")},
	}

	if shouldCountNetworkInterface(iface) {
		t.Fatalf("expected TAP adapter to be excluded")
	}
}

func TestShouldCountNetworkInterfaceRejectsLinkLocalOnly(t *testing.T) {
	iface := networkInterfaceInfo{
		Name:         "Ethernet0",
		Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
		HardwareAddr: stdnet.HardwareAddr{0x00, 0x50, 0x56, 0xba, 0x25, 0x01},
		Addrs:        []stdnet.Addr{mustCIDR(t, "fe80::c562:f8dc:9cd4:18eb/64")},
	}

	if shouldCountNetworkInterface(iface) {
		t.Fatalf("expected link-local-only interface to be excluded")
	}
}

func TestShouldCountNetworkInterfaceRejectsLoopback(t *testing.T) {
	iface := networkInterfaceInfo{
		Name:  "Loopback Pseudo-Interface 1",
		Flags: stdnet.FlagUp | stdnet.FlagLoopback,
		Addrs: []stdnet.Addr{mustCIDR(t, "127.0.0.1/8")},
	}

	if shouldCountNetworkInterface(iface) {
		t.Fatalf("expected loopback interface to be excluded")
	}
}

func TestBuildCountedInterfaceSetIncludesOnlyEligibleNames(t *testing.T) {
	interfaces := []networkInterfaceInfo{
		{
			Name:         "Ethernet0",
			Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
			HardwareAddr: stdnet.HardwareAddr{0x00, 0x50, 0x56, 0xba, 0x25, 0x01},
			Addrs:        []stdnet.Addr{mustCIDR(t, "10.100.1.10/28")},
		},
		{
			Name:         "TAP-Windows Adapter V9",
			Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
			HardwareAddr: stdnet.HardwareAddr{0x00, 0xff, 0x55, 0x61, 0x3a, 0xd2},
			Addrs:        []stdnet.Addr{mustCIDR(t, "10.8.0.2/24")},
		},
	}

	counted := buildCountedInterfaceSet(interfaces)
	if !counted["Ethernet0"] {
		t.Fatalf("expected Ethernet0 to be counted")
	}
	if counted["TAP-Windows Adapter V9"] {
		t.Fatalf("expected TAP adapter to be excluded")
	}
}

func TestBuildCountedInterfaceSetCountsBondOnce(t *testing.T) {
	bondMAC := stdnet.HardwareAddr{0x52, 0x54, 0x00, 0x12, 0x34, 0x56}
	interfaces := []networkInterfaceInfo{
		{
			Name:         "bond0",
			Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
			HardwareAddr: bondMAC,
			Addrs:        []stdnet.Addr{mustCIDR(t, "192.168.10.20/24")},
		},
		{
			Name:         "eno1",
			Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
			HardwareAddr: bondMAC,
		},
		{
			Name:         "eno2",
			Flags:        stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast,
			HardwareAddr: bondMAC,
		},
	}

	counted := buildCountedInterfaceSet(interfaces)
	if !counted["bond0"] {
		t.Fatalf("expected bond0 carrying the host IP to be counted")
	}
	if counted["eno1"] || counted["eno2"] {
		t.Fatalf("expected bond slaves without IPs to be excluded so traffic is not double counted")
	}
}
