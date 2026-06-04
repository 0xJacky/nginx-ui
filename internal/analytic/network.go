package analytic

import (
	stdnet "net"
	"strings"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/uozi-tech/cosy/logger"
)

type networkInterfaceInfo struct {
	Name         string
	Flags        stdnet.Flags
	HardwareAddr stdnet.HardwareAddr
	Addrs        []stdnet.Addr
}

func shouldCountNetworkInterface(iface networkInterfaceInfo) bool {
	if iface.Flags&stdnet.FlagUp == 0 || iface.Flags&stdnet.FlagLoopback != 0 {
		return false
	}

	if isVirtualInterface(iface.Name) {
		return false
	}

	if len(iface.HardwareAddr) == 0 {
		return false
	}

	return hasUsableUnicastIP(iface.Addrs)
}

func buildCountedInterfaceSet(interfaces []networkInterfaceInfo) map[string]bool {
	countedInterfaces := make(map[string]bool)
	for _, iface := range interfaces {
		if shouldCountNetworkInterface(iface) {
			countedInterfaces[iface.Name] = true
		}
	}
	return countedInterfaces
}

func hasUsableUnicastIP(addrs []stdnet.Addr) bool {
	for _, addr := range addrs {
		ip, _, err := stdnet.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if !ip.IsGlobalUnicast() {
			continue
		}

		if ip.IsLinkLocalUnicast() || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
			continue
		}

		if isReservedIP(ip) {
			continue
		}

		return true
	}

	return false
}

func GetNetworkStat() (data *net.IOCountersStat, err error) {
	networkStats, err := net.IOCounters(true)
	if err != nil {
		return
	}
	if len(networkStats) == 0 {
		return &net.IOCountersStat{}, nil
	}
	// Get all network interfaces
	interfaces, err := stdnet.Interfaces()
	if err != nil {
		logger.Error(err)
		return
	}

	var (
		totalBytesRecv   uint64
		totalBytesSent   uint64
		totalPacketsRecv uint64
		totalPacketsSent uint64
		totalErrIn       uint64
		totalErrOut      uint64
		totalDropIn      uint64
		totalDropOut     uint64
		totalFifoIn      uint64
		totalFifoOut     uint64
	)

	interfaceInfos := make([]networkInterfaceInfo, 0, len(interfaces))
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Error(err)
			continue
		}

		interfaceInfos = append(interfaceInfos, networkInterfaceInfo{
			Name:         iface.Name,
			Flags:        iface.Flags,
			HardwareAddr: iface.HardwareAddr,
			Addrs:        addrs,
		})
	}

	countedInterfaces := buildCountedInterfaceSet(interfaceInfos)

	// Accumulate stats only from counted interfaces
	for _, stat := range networkStats {
		if countedInterfaces[stat.Name] {
			totalBytesRecv += stat.BytesRecv
			totalBytesSent += stat.BytesSent
			totalPacketsRecv += stat.PacketsRecv
			totalPacketsSent += stat.PacketsSent
			totalErrIn += stat.Errin
			totalErrOut += stat.Errout
			totalDropIn += stat.Dropin
			totalDropOut += stat.Dropout
			totalFifoIn += stat.Fifoin
			totalFifoOut += stat.Fifoout
		}
	}

	return &net.IOCountersStat{
		Name:        "analytic.network",
		BytesRecv:   totalBytesRecv,
		BytesSent:   totalBytesSent,
		PacketsRecv: totalPacketsRecv,
		PacketsSent: totalPacketsSent,
		Errin:       totalErrIn,
		Errout:      totalErrOut,
		Dropin:      totalDropIn,
		Dropout:     totalDropOut,
		Fifoin:      totalFifoIn,
		Fifoout:     totalFifoOut,
	}, nil
}

// isVirtualInterface checks if the interface is a virtual one based on name patterns
func isVirtualInterface(name string) bool {
	// Common virtual interface name patterns
	virtualPatterns := []string{
		"veth", "virbr", "vnet", "vmnet", "vboxnet", "docker",
		"br-", "bridge", "tun", "tap", "bond", "dummy",
		"vpn", "ipsec", "gre", "sit", "vlan", "virt",
		"wg", "vmk", "ib", "vxlan", "geneve", "ovs",
		"hyperv", "hyper-v", "awdl", "llw", "utun",
		"vpn", "zt", "zerotier", "wireguard",
	}

	for _, pattern := range virtualPatterns {
		if strings.Contains(strings.ToLower(name), pattern) {
			return true
		}
	}

	return false
}

// isReservedIP checks if an IP belongs to special reserved ranges
func isReservedIP(ip stdnet.IP) bool {
	// Handle IPv4
	if ip4 := ip.To4(); ip4 != nil {
		// TEST-NET-1: 192.0.2.0/24 (RFC 5737)
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
			return true
		}

		// TEST-NET-2: 198.51.100.0/24 (RFC 5737)
		if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
			return true
		}

		// TEST-NET-3: 203.0.113.0/24 (RFC 5737)
		if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
			return true
		}

		// Benchmark tests: 198.18.0.0/15 (includes 198.19.0.0/16) (RFC 2544)
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return true
		}

		// Documentation: 240.0.0.0/4 (RFC 1112)
		if ip4[0] >= 240 {
			return true
		}

		// CGNAT: 100.64.0.0/10 (RFC 6598)
		if ip4[0] == 100 && (ip4[1]&0xC0) == 64 {
			return true
		}
	} else if ip.To16() != nil {
		// Documentation prefix (2001:db8::/32) - RFC 3849
		if ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
			return true
		}

		// Unique Local Addresses (fc00::/7) - RFC 4193
		if (ip[0] & 0xfe) == 0xfc {
			return true
		}

		// 6to4 relay (2002::/16) - RFC 3056
		if ip[0] == 0x20 && ip[1] == 0x02 {
			return true
		}

		// Teredo tunneling (2001:0::/32) - RFC 4380
		if ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x00 && ip[3] == 0x00 {
			return true
		}

		// Deprecated site-local addresses (fec0::/10) - RFC 3879
		if (ip[0]&0xff) == 0xfe && (ip[1]&0xc0) == 0xc0 {
			return true
		}

		// Old 6bone addresses (3ffe::/16) - Deprecated
		if ip[0] == 0x3f && ip[1] == 0xfe {
			return true
		}

		// ORCHID addresses (2001:10::/28) - RFC 4843
		if ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x00 && (ip[3]&0xf0) == 0x10 {
			return true
		}

		// ORCHID v2 addresses (2001:20::/28) - RFC 7343
		if ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x00 && (ip[3]&0xf0) == 0x20 {
			return true
		}
	}

	return false
}
