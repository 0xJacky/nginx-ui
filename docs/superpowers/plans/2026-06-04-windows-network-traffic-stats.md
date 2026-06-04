# Windows Network Traffic Stats Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix Windows network traffic statistics showing zero when the active adapter is named like `Ethernet0` and uses a private IPv4 address.

**Architecture:** Keep the fix inside `internal/analytic/network.go`. Extract the interface selection rule into a small testable helper, then let `GetNetworkStat()` aggregate any up, non-loopback, non-virtual interface with a hardware address and at least one usable unicast IP address, including private IPv4 ranges. Continue excluding disconnected, loopback, link-local-only, TAP, tunnel, VPN, Docker, and similar virtual interfaces.

**Tech Stack:** Go, `net`, `github.com/shirou/gopsutil/v4/net`, existing `go test` workflow.

---

## File Structure

- Modify: `internal/analytic/network.go`
  - Keep `GetNetworkStat()` as the public entry point.
  - Add a lightweight `networkInterfaceInfo` struct so selection logic can be unit-tested without depending on host NICs.
  - Replace the current public-IP-only gate with `hasUsableUnicastIP()`.
- Create: `internal/analytic/network_test.go`
  - Unit tests for Windows-style adapter names and private IPs.
  - Regression tests for virtual, loopback, down, and link-local-only interfaces.

---

### Task 1: Add Failing Network Interface Selection Tests

**Files:**
- Create: `internal/analytic/network_test.go`

- [ ] **Step 1: Create tests for the Windows regression and exclusions**

Create `internal/analytic/network_test.go`:

```go
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
```

- [ ] **Step 2: Run tests and verify they fail because the helper does not exist**

Run:

```bash
go test ./internal/analytic
```

Expected: FAIL with compile errors containing `undefined: networkInterfaceInfo` and `undefined: shouldCountNetworkInterface`.

---

### Task 2: Implement Testable Interface Classification

**Files:**
- Modify: `internal/analytic/network.go`

- [ ] **Step 1: Add testable interface metadata and selection helpers**

Add this near the top of `internal/analytic/network.go`, after imports and before `GetNetworkStat()`:

```go
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
```

- [ ] **Step 2: Run tests and verify helper behavior passes but `GetNetworkStat()` still uses old selection**

Run:

```bash
go test ./internal/analytic
```

Expected: PASS for the new helper tests if they are the only changed tests; the runtime path has not been updated yet.

---

### Task 3: Wire `GetNetworkStat()` to the New Selection Rule

**Files:**
- Modify: `internal/analytic/network.go`

- [ ] **Step 1: Replace the existing external-interface discovery loop**

In `GetNetworkStat()`, replace the current logic that checks `isPhysicalInterface()` and `isRealExternalIP()` with this loop:

```go
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
```

Then update the aggregation condition from:

```go
if externalInterfaces[stat.Name] {
```

to:

```go
if countedInterfaces[stat.Name] {
```

- [ ] **Step 2: Remove now-unused physical/public-IP selection helpers**

Remove these functions if they are no longer referenced:

```go
func isPhysicalInterface(name string) bool
func isNumericSuffix(s string) bool
func isRealExternalIP(ip stdnet.IP, ipNet *stdnet.IPNet) bool
```

Keep `isReservedIP()` because `hasUsableUnicastIP()` still uses it.

- [ ] **Step 3: Run gofmt**

Run:

```bash
gofmt -w internal/analytic/network.go internal/analytic/network_test.go
```

Expected: no output.

- [ ] **Step 4: Run analytic package tests**

Run:

```bash
go test ./internal/analytic
```

Expected: PASS.

---

### Task 4: Add Regression Coverage for Aggregation Name Matching

**Files:**
- Modify: `internal/analytic/network_test.go`

- [ ] **Step 1: Add a test for building the counted interface map**

Add this test:

```go
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
```

- [ ] **Step 2: Run tests**

Run:

```bash
go test ./internal/analytic
```

Expected: PASS.

---

### Task 5: Final Verification

**Files:**
- Verify only; no edits.

- [ ] **Step 1: Check the changed file list**

Run:

```bash
git diff --stat -- internal/analytic/network.go internal/analytic/network_test.go
```

Expected: only the network stat implementation and its tests are listed.

- [ ] **Step 2: Run package tests**

Run:

```bash
go test ./internal/analytic
```

Expected: PASS.

- [ ] **Step 3: Optional broader backend smoke test**

Run:

```bash
go test ./api/analytic ./internal/analytic
```

Expected: PASS.

---

## Self-Review

- Spec coverage: The plan addresses the observed Windows Server 2012 R2 output where `Ethernet0` has a private `10.100.x.x` address and traffic is currently filtered to zero.
- Risk control: The plan keeps virtual/tunnel exclusions and avoids switching blindly to `net.IOCounters(false)`, which could count loopback and virtual adapters.
- Test coverage: The plan adds regression tests for the Windows physical adapter case and for the exclusion cases that the old filter tried to protect.
