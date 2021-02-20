package tool

import (
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
)

func ReloadNginx() {
	cmd := exec.Command("systemctl", "reload nginx")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Println(err)
	}

	log.Println(out.String())
}

func GetNginxConfPath(dir string) string {
	return filepath.Join("/etc/nginx", dir)
}
