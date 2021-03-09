package tool

import (
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
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
	out, err := exec.Command("nginx", "-V").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Printf("%s\n", out)

	r, _ := regexp.Compile("--conf-path=(.*)/(.*.conf)")

	confPath := r.FindStringSubmatch(string(out))[1]

	// fmt.Println(confPath)

	return filepath.Join(confPath, dir)
}
