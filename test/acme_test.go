package test

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestAcme(t *testing.T) {
	const acmePath = "/usr/local/acme.sh"
	_, err := os.Stat(acmePath)
	log.Println("[found] acme.sh ", acmePath)
	if err != nil {
		log.Println(err)
		if os.IsNotExist(err) {
			log.Println("[not found] acme.sh, installing...")

			if _, err := os.Stat("../tmp"); os.IsNotExist(err) {
				_ = os.Mkdir("../tmp", 0644)
			}

			out, err := exec.Command("curl", "-o", "../tmp/acme.sh", "https://get.acme.sh").
				CombinedOutput()
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Printf("%s\n", out)

			log.Println("[acme.sh] downloaded")

			file, _ := ioutil.ReadFile("../tmp/acme.sh")

			fileString := string(file)
			fileString = strings.Replace(fileString, "https://raw.githubusercontent.com",
				"https://mirror.ghproxy.com/https://raw.githubusercontent.com", -1)

			_ = ioutil.WriteFile("../tmp/acme.sh", []byte(fileString), 0644)

			out, err = exec.Command("bash", "../tmp/acme.sh",
				"install",
				"--log",
				"--home", "/usr/local/acme.sh",
				"--cert-home", nginx.GetConfPath("ssl")).
				CombinedOutput()
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Printf("%s\n", out)

		}
	}
}
