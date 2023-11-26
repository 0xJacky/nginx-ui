package test

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"log"
	"os"
	"os/exec"
	"testing"
)

func TestCert(t *testing.T) {
	out, err := exec.Command("bash", "/usr/local/acme.sh/acme.sh",
		"--issue",
		"-d", "test.ojbk.me",
		"--nginx").CombinedOutput()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("%s\n", out)

	_, err = os.Stat(nginx.GetConfPath("ssl/test.ojbk.me/fullchain.cer"))

	if err != nil {
		log.Println(err)
		return
	}
	log.Println("[found]", "fullchain.cer")
	_, err = os.Stat(nginx.GetConfPath("ssl/test.ojbk.me/test.ojbk.me.key"))

	if err != nil {
		log.Println(err)
		return
	}

	log.Println("[found]", "cert key")
}
