package setup

import (
	"encoding/json"
	"strings"
	"testing"
)

// The wizard reads ok, level, detail and remediation straight off the wire, so
// the JSON shape is a contract rather than an implementation detail.
func TestVerifyResultMarshalsTheStepContract(t *testing.T) {
	raw, err := json.Marshal(VerifyResult{Steps: map[string]StepOutcome{
		"ssh_connect": {OK: true, Detail: "ok"},
		"nginx_test":  {OK: false, Level: "warning", Detail: "skipped", Remediation: "run it"},
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
	if !strings.Contains(outcome.Remediation, "/etc/nginx-ui") {
		t.Fatalf("expected remediation to mention /etc/nginx-ui: %q", outcome.Remediation)
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
