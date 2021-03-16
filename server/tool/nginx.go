package tool

import (
    "log"
    "os/exec"
    "path/filepath"
    "regexp"
)

func ReloadNginx() string {
	out, err := exec.Command("nginx", "-s", "reload").CombinedOutput()

	if err != nil {
        log.Println(err)
    }

    output := string(out)
	log.Println(output)

    return output
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
