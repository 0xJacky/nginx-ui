package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/cert/config"
	"github.com/BurntSushi/toml"
	"log"
	"strings"
	"testing"
)

func CheckIfErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func TestConfigEnv(t *testing.T) {
	filenames, err := config.ListConfigs()
	CheckIfErr(err)

	for _, filename := range filenames {
		if !strings.HasSuffix(filename, ".toml") {
			continue
		}
		
		data, err := config.GetConfig(filename)
		CheckIfErr(err)

		c := Config{}
		err = toml.Unmarshal(data, &c)
		CheckIfErr(err)

		log.Println(c.Name)

		if c.Configuration != nil {
			for k, v := range c.Configuration.Credentials {
				log.Println(k, v)
			}

			for k, v := range c.Configuration.Additional {
				log.Println(k, v)
			}
		}

		if c.Links != nil {
			log.Println(c.Links.API)
			log.Println(c.Links.GoClient)
		}
	}
}
