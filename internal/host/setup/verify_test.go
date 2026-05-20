package setup

import "testing"

func TestVerifyResult_Marshal(t *testing.T) {
	r := VerifyResult{Steps: map[string]StepOutcome{
		"ssh_connect": {OK: true, Detail: "ok"},
	}}
	if r.Steps["ssh_connect"].OK != true {
		t.Errorf("expected ok=true")
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
