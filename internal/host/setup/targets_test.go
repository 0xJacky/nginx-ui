package setup

import (
	"context"
	"net"
	"slices"
	"testing"
)

func TestParseHexIPv4(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "little endian gateway", value: "016BA8C0", want: "192.168.107.1"},
		{name: "unspecified is dropped", value: "00000000", want: ""},
		{name: "malformed hex is dropped", value: "zz", want: ""},
		{name: "wrong length is dropped", value: "0102", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseHexIPv4(tc.value); got != tc.want {
				t.Fatalf("parseHexIPv4(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestProbeTCPReportsReachability(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	if !probeTCP(context.Background(), listener.Addr().String()) {
		t.Fatal("an open port must be reported reachable")
	}

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := closed.Addr().String()
	_ = closed.Close()
	if probeTCP(context.Background(), address) {
		t.Fatal("a closed port must be reported unreachable")
	}
}

func TestCollectSSHTargetCandidatesOnlyGuessTheDefaultPort(t *testing.T) {
	candidates := collectSSHTargetCandidates(nil)
	// Without this the loop below can execute zero times and still pass on a
	// machine with no docker alias and no default route.
	if len(candidates) == 0 {
		t.Skip("no discoverable candidate on this machine")
	}

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		if _, ok := seen[candidate.Address]; ok {
			t.Fatalf("duplicate candidate %q", candidate.Address)
		}
		seen[candidate.Address] = struct{}{}

		_, port, err := net.SplitHostPort(candidate.Address)
		if err != nil {
			t.Fatalf("candidate %q is not host:port: %v", candidate.Address, err)
		}
		if port != defaultSSHPort {
			t.Fatalf("candidate %q guesses a port other than %s", candidate.Address, defaultSSHPort)
		}
		if candidate.Source == "" {
			t.Fatalf("candidate %q has no source", candidate.Address)
		}
	}
}

func TestCollectSSHTargetCandidatesKeepsARequestedPort(t *testing.T) {
	candidates := collectSSHTargetCandidates([]string{"192.168.50.165:2222"})

	var found bool
	for _, candidate := range candidates {
		if candidate.Address == "192.168.50.165:2222" {
			found = true
			if candidate.Source != "requested" {
				t.Fatalf("source = %q, want requested", candidate.Source)
			}
		}
	}
	if !found {
		t.Fatalf("the requested address was dropped: %+v", candidates)
	}
}

func TestCollectSSHTargetCandidatesAddsTheDefaultPortToARequestedHost(t *testing.T) {
	candidates := collectSSHTargetCandidates([]string{"example.internal"})

	want := net.JoinHostPort("example.internal", defaultSSHPort)
	if !slices.ContainsFunc(candidates, func(c SSHTarget) bool { return c.Address == want }) {
		t.Fatalf("candidate %q is missing: %+v", want, candidates)
	}
}
