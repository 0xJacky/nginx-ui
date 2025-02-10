package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/cert/config"
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"path/filepath"
)

type Configuration struct {
	Credentials map[string]string `json:"credentials"`
	Additional  map[string]string `json:"additional"`
}

type Links struct {
	API      string `json:"api"`
	GoClient string `json:"go_client"`
}

type Config struct {
	Name          string         `json:"name"`
	Code          string         `json:"code"`
	Configuration *Configuration `json:"configuration,omitempty"`
	Links         *Links         `json:"links,omitempty"`
}

var configurations []Config

var configurationMap map[string]Config

func init() {
	files, err := config.DistFS.ReadDir(".")

	if err != nil {
		log.Fatalln(err)
	}

	configurationMap = make(map[string]Config)

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".toml" {
			continue
		}
		c := Config{}

		_, err = toml.DecodeFS(config.DistFS, file.Name(), &c)

		if err != nil {
			log.Fatalln(err)
		}

		configurationMap[c.Code] = c

		c.Configuration = nil
		c.Links = nil
		configurations = append(configurations, c)
	}
}

func GetProvidersList() []Config {
	return configurations
}

func GetProvider(code string) (Config, bool) {
	if v, ok := configurationMap[code]; ok {
		return v, ok
	}
	return Config{}, false
}

func (c *Config) SetEnv(configuration Configuration) error {
	if c.Configuration != nil {
		for k := range c.Configuration.Credentials {
			if value, ok := configuration.Credentials[k]; ok {
				err := os.Setenv(k, value)
				if err != nil {
					return err
				}
			}
		}
		for k := range c.Configuration.Additional {
			if value, ok := configuration.Additional[k]; ok {
				err := os.Setenv(k, value)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Config) CleanEnv() {
	if c.Configuration != nil {
		for k := range c.Configuration.Credentials {
			_ = os.Unsetenv(k)
		}
		for k := range c.Configuration.Additional {
			_ = os.Unsetenv(k)
		}
	}
}
