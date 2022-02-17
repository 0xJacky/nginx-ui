package test

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"testing"
)

func TestGetNginx(t *testing.T)  {
	out, err := exec.Command("nginx", "-V").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", out)

	r, _ := regexp.Compile("--conf-path=(.*)/(.*.conf)")
	fmt.Println(r.FindStringSubmatch(string(out))[1])
}
