package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/cert/config"
	"github.com/BurntSushi/toml"
	"log"
	"path/filepath"
	"testing"
)

func CheckIfErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func TestConfigEnv(t *testing.T) {

	files, err := config.DistFS.ReadDir(".")

	CheckIfErr(err)

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".toml" {
			continue
		}
		c := Config{}

		_, err := toml.DecodeFS(config.DistFS, file.Name(), &c)
		CheckIfErr(err)

		log.Println(c.Name)

		for k, v := range c.Configuration.Credentials {
			log.Println(k, v)
		}

		for k, v := range c.Configuration.Additional {
			log.Println(k, v)
		}
	}

}
