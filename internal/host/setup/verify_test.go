package setup

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The wizard reads ok, level, detail and the remediation message contract
// straight off the wire, so the JSON shape is not an implementation detail.
func TestVerifyResultMarshalsTheStepContract(t *testing.T) {
	raw, err := json.Marshal(VerifyResult{Steps: map[string]StepOutcome{
		"ssh_connect": {OK: true, Detail: "ok"},
		"nginx_test": {OK: false, Level: "warning", Detail: "skipped", Remediation: newStepRemediation(
			remediationAddBindMount,
			map[string]string{"source": "/host/nginx", "target": "/etc/nginx"},
		)},
	}})
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Steps map[string]map[string]any `json:"steps"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	connect := decoded.Steps["ssh_connect"]
	if connect["ok"] != true {
		t.Fatalf("ok = %v, want true", connect["ok"])
	}
	// level and remediation are omitempty, so a plain pass must not carry them.
	if _, present := connect["level"]; present {
		t.Fatalf("a plain pass serialised a level: %s", raw)
	}
	if _, present := connect["remediation"]; present {
		t.Fatalf("a plain pass serialised a remediation: %s", raw)
	}

	skipped := decoded.Steps["nginx_test"]
	if skipped["ok"] != false || skipped["level"] != "warning" {
		t.Fatalf("a skipped step lost its contract: %s", raw)
	}
	remediation, ok := skipped["remediation"].(map[string]any)
	if !ok || remediation["code"] != remediationAddBindMount {
		t.Fatalf("remediation message code was not serialised: %s", raw)
	}
	params, ok := remediation["params"].(map[string]any)
	if !ok || params["source"] != "/host/nginx" || params["target"] != "/etc/nginx" {
		t.Fatalf("remediation message parameters were not serialised: %s", raw)
	}
}

func TestParseSudoListOutput_FindsMatches(t *testing.T) {
	out := `User nginxui may run the following commands on host:
    (root) NOPASSWD: /bin/systemctl reload nginx.service
    (root) NOPASSWD: /bin/systemctl restart nginx.service
    (root) NOPASSWD: /usr/sbin/nginx -t
    (root) NOPASSWD: /usr/sbin/nginx -T`
	required := []string{
		"/bin/systemctl reload nginx.service",
		"/bin/systemctl restart nginx.service",
		"/usr/sbin/nginx -t",
		"/usr/sbin/nginx -T",
	}
	missing := findMissingSudoEntries(out, required)
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestParseSudoListOutput_FindsMissing(t *testing.T) {
	out := `User nginxui may run the following commands on host:
    (root) NOPASSWD: /bin/systemctl reload nginx.service`
	required := []string{
		"/bin/systemctl reload nginx.service",
		"/bin/systemctl restart nginx.service",
	}
	missing := findMissingSudoEntries(out, required)
	if len(missing) != 1 || missing[0] != "/bin/systemctl restart nginx.service" {
		t.Errorf("expected restart missing, got %v", missing)
	}
}

func TestCheckKnownHostsPersistenceRecommended(t *testing.T) {
	outcome := checkKnownHostsPersistence("/etc/nginx-ui/known_hosts")
	if !outcome.OK {
		t.Fatalf("expected recommended path to pass: %+v", outcome)
	}
	if outcome.Level != "success" {
		t.Fatalf("expected success level, got %q", outcome.Level)
	}
}

func TestCheckKnownHostsPersistenceWarning(t *testing.T) {
	outcome := checkKnownHostsPersistence("/tmp/known_hosts")
	if outcome.OK {
		t.Fatalf("expected outside path to be non-ok warning: %+v", outcome)
	}
	if outcome.Level != "warning" {
		t.Fatalf("expected warning level, got %q", outcome.Level)
	}
	if outcome.Remediation == nil || outcome.Remediation.Code != remediationPersistKnownHosts {
		t.Fatalf("unexpected remediation message: %+v", outcome.Remediation)
	}
}

func TestCheckLogReadableUsesCrossPlatformRemediation(t *testing.T) {
	outcome := checkLogReadable("/path/that/does/not/exist/access.log")
	if outcome.OK {
		t.Fatalf("missing log unexpectedly passed: %+v", outcome)
	}
	if outcome.Remediation == nil || outcome.Remediation.Code != remediationMountNginxLogs {
		t.Fatalf("unexpected remediation message: %+v", outcome.Remediation)
	}
}

func TestMountedHostPathFor(t *testing.T) {
	mountInfo := []byte(`841 817 0:32 / / rw,relatime - overlay overlay rw
854 841 0:36 /opt/homebrew/etc/nginx /opt/homebrew/etc/nginx rw,relatime - virtiofs mac rw
855 841 0:37 /opt/homebrew/var /opt/homebrew/var rw,relatime - virtiofs mac rw`)

	tests := []struct {
		name   string
		target string
		want   string
		ok     bool
	}{
		{"exact mount", "/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"directory under parent mount", "/opt/homebrew/var/log/nginx", "/opt/homebrew/var/log/nginx", true},
		{"root filesystem only", "/etc/nginx", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mountedHostPathFor(mountInfo, tt.target)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("mountedHostPathFor(%q) = %q, %v; want %q, %v", tt.target, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestVirtualizedMountRootMatches(t *testing.T) {
	for _, tt := range []struct {
		name, mounted, host string
		want                bool
	}{
		{"native root", "/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"vm prefix", "/host_mnt/opt/homebrew/etc/nginx", "/opt/homebrew/etc/nginx", true},
		{"wrong source", "/private/tmp/nginx", "/opt/homebrew/etc/nginx", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := virtualizedMountRootMatches(tt.mounted, tt.host); got != tt.want {
				t.Fatalf("virtualizedMountRootMatches(%q, %q) = %v, want %v", tt.mounted, tt.host, got, tt.want)
			}
		})
	}
}

func TestCheckMacOSHostGateway(t *testing.T) {
	resolved := checkMacOSHostGateway("host.docker.internal:22", func(host string) ([]string, error) {
		if host != "host.docker.internal" {
			t.Fatalf("lookup host = %q", host)
		}
		return []string{"0.250.250.254"}, nil
	})
	if !resolved.OK || !strings.Contains(resolved.Detail, "0.250.250.254") {
		t.Fatalf("resolved macOS gateway did not pass: %+v", resolved)
	}

	unresolved := checkMacOSHostGateway("host.docker.internal:22", func(string) ([]string, error) {
		return nil, errors.New("not found")
	})
	if unresolved.OK || unresolved.Level != "warning" {
		t.Fatalf("unresolved macOS gateway did not warn: %+v", unresolved)
	}

	wrongAlias := checkMacOSHostGateway("192.168.1.10:22", func(string) ([]string, error) {
		return []string{"192.168.1.10"}, nil
	})
	if wrongAlias.OK || wrongAlias.Remediation == nil || wrongAlias.Remediation.Code != remediationUseMacOSHostAlias {
		t.Fatalf("non-standard alias unexpectedly passed: %+v", wrongAlias)
	}
}

func TestParseCheckGroupsDropsUnknownValues(t *testing.T) {
	groups := ParseCheckGroups([]string{"platform", "bogus", "nginx"})
	if len(groups) != 2 || groups[0] != CheckGroupPlatform || groups[1] != CheckGroupNginx {
		t.Fatalf("groups = %v, want [platform nginx]", groups)
	}
	if ParseCheckGroups(nil) != nil {
		t.Fatal("an empty request must stay empty so the full pipeline runs")
	}
}

func TestCheckGroupFilter(t *testing.T) {
	all := newCheckGroupFilter(nil)
	for _, group := range AllCheckGroups {
		if !all(group) {
			t.Fatalf("empty filter must accept %q", group)
		}
	}

	only := newCheckGroupFilter([]CheckGroup{CheckGroupPlatform})
	if !only(CheckGroupPlatform) {
		t.Fatal("selected group must be accepted")
	}
	for _, group := range []CheckGroup{CheckGroupConnection, CheckGroupPrivileges, CheckGroupNginx} {
		if only(group) {
			t.Fatalf("unselected group %q must be rejected", group)
		}
	}
}

// systemctl always prints the property name, so the old check could never see
// an undeclared ExecReload and always reported a pass.
func TestExecReloadValueDecidesTheOutcome(t *testing.T) {
	cases := []struct {
		name   string
		output string
		wantOK bool
	}{
		{"declared", "ExecReload={ path=/bin/kill ; argv[]=/bin/kill -s HUP $MAINPID }", true},
		{"undeclared", "ExecReload=", false},
		{"undeclared with trailing space", "ExecReload= \n", false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tt.output), "ExecReload="))
			if got := value != ""; got != tt.wantOK {
				t.Fatalf("ExecReload %q declared = %v, want %v", tt.output, got, tt.wantOK)
			}
		})
	}
}

// An all-unknown group request must stay narrow rather than running everything.
func TestCheckGroupFilterDoesNotWidenForUnknownGroups(t *testing.T) {
	unknown := ParseCheckGroups([]string{"not-a-group"})
	if unknown == nil {
		t.Fatal("ParseCheckGroups returned nil for a named but unmatched request")
	}

	filter := newCheckGroupFilter(unknown)
	for _, group := range AllCheckGroups {
		if filter(group) {
			t.Fatalf("group %q ran for an all-unknown request", group)
		}
	}

	full := newCheckGroupFilter(ParseCheckGroups(nil))
	for _, group := range AllCheckGroups {
		if !full(group) {
			t.Fatalf("group %q was skipped for an empty request", group)
		}
	}
}
