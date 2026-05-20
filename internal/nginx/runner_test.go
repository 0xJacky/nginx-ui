package nginx

import (
	"context"
	"os"
	"testing"
)

func TestLocalRunner_Stat(t *testing.T) {
	r := &localRunner{}
	tmp, err := os.CreateTemp("", "runner-stat-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	if !r.Stat(tmp.Name()) {
		t.Errorf("Stat(%q) = false, want true", tmp.Name())
	}
	if r.Stat("/nonexistent/path/that/should/not/exist") {
		t.Errorf("Stat(nonexistent) = true, want false")
	}
}

func TestLocalRunner_Exec_Echo(t *testing.T) {
	r := &localRunner{}
	out, err := r.Exec(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Exec returned err: %v", err)
	}
	if want := "hello\n"; out != want && out != "hello\r\n" {
		t.Errorf("Exec output = %q, want %q", out, want)
	}
}

func TestDockerRunner_RoutesToDockerExec(t *testing.T) {
	// We can't actually exercise docker.Exec without a docker daemon,
	// so this is a smoke test ensuring the type satisfies the interface.
	var _ Runner = (*dockerRunner)(nil)
}
