package setup

import (
	"context"
	"fmt"
	"testing"
)

type discoveryExecutor struct {
	outputs map[string]string
}

func (e discoveryExecutor) Exec(_ context.Context, name string, args ...string) (string, error) {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	if output, ok := e.outputs[key]; ok {
		return output, nil
	}
	return "", fmt.Errorf("not found: %s", key)
}

func TestDiscoverNginxHomebrewAppleSilicon(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/opt/homebrew/bin/brew --prefix nginx": "/opt/homebrew/opt/nginx\n",
		"/opt/homebrew/opt/nginx/bin/nginx -V":  "nginx version: nginx/1.31.3\nconfigure arguments: --prefix=/opt/homebrew/Cellar/nginx/1.31.3 --conf-path=/opt/homebrew/etc/nginx/nginx.conf --pid-path=/opt/homebrew/var/run/nginx.pid --http-log-path=/opt/homebrew/var/log/nginx/access.log --error-log-path=/opt/homebrew/var/log/nginx/error.log\n",
	}}
	result, err := DiscoverNginx(context.Background(), executor, SetupParams{
		ServiceManager: "launchd",
		NginxSbinPath:  "/missing/nginx",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "nginx/1.31.3" || result.ExecutablePath != "/opt/homebrew/opt/nginx/bin/nginx" {
		t.Fatalf("unexpected binary discovery: %+v", result)
	}
	if result.ConfigDir != "/opt/homebrew/etc/nginx" || result.LogDir != "/opt/homebrew/var/log/nginx" {
		t.Fatalf("unexpected directory discovery: %+v", result)
	}
	if result.DocumentRoot != "/opt/homebrew/var/www" || result.PIDPath != "/opt/homebrew/var/run/nginx.pid" {
		t.Fatalf("unexpected Homebrew paths: %+v", result)
	}
}

func TestDiscoverNginxHomebrewIntelFallback(t *testing.T) {
	executor := discoveryExecutor{outputs: map[string]string{
		"/usr/local/bin/nginx -V": "nginx version: nginx/1.27.4\nconfigure arguments: --prefix=/usr/local/Cellar/nginx/1.27.4 --conf-path=/usr/local/etc/nginx/nginx.conf --pid-path=/usr/local/var/run/nginx.pid --http-log-path=/usr/local/var/log/nginx/access.log --error-log-path=/usr/local/var/log/nginx/error.log\n",
	}}
	result, err := DiscoverNginx(context.Background(), executor, SetupParams{ServiceManager: "launchd"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigDir != "/usr/local/etc/nginx" || result.DocumentRoot != "/usr/local/var/www" {
		t.Fatalf("unexpected Intel Homebrew discovery: %+v", result)
	}
}

func TestParseNginxVersionOutputResolvesRelativeCompilePaths(t *testing.T) {
	result := parseNginxVersionOutput("/custom/nginx", "nginx version: nginx/1.26.0\nconfigure arguments: --prefix=/srv/nginx --conf-path='conf/nginx.conf' --pid-path=run/nginx.pid\n", SetupParams{})
	if result.ConfigPath != "/srv/nginx/conf/nginx.conf" || result.PIDPath != "/srv/nginx/run/nginx.pid" {
		t.Fatalf("relative paths were not resolved: %+v", result)
	}
}
