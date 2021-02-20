package main

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/router"
	"log"
	"os"
	"os/exec"
	"strings"
)

func getCurrentPath() string {
	s, err := exec.LookPath(os.Args[0])
	if err != nil {
		log.Println(err)
	}
	i := strings.LastIndex(s, "\\")
	path := string(s[0 : i+1])
	return path
}


func main() {
	r := router.InitRouter()

	model.Setup()

	r.Run(":9000") // listen and serve on 0.0.0.0:9000
}
