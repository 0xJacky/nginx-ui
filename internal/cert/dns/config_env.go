package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/cert/config"
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
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
	envBackup     map[string]*string
}

var configurations []Config

var configurationMap map[string]Config

func init() {
	filenames, err := config.ListConfigs()
	if err != nil {
		log.Fatalln(err)
	}

	configurationMap = make(map[string]Config)

	for _, filename := range filenames {
		if !strings.HasSuffix(filename, ".toml") {
			continue
		}

		data, err := config.GetConfig(filename)
		if err != nil {
			log.Fatalln(err)
		}

		c := Config{}
		err = toml.Unmarshal(data, &c)
		if err != nil {
			log.Fatalln(err)
		}

		configurationMap[c.Code] = c

		c.Configuration = nil
		c.Links = nil
		configurations = append(configurations, c)
	}

	sort.SliceStable(configurations, func(i, j int) bool {
		leftName := strings.ToLower(configurations[i].Name)
		rightName := strings.ToLower(configurations[j].Name)
		if leftName == rightName {
			return strings.ToLower(configurations[i].Code) < strings.ToLower(configurations[j].Code)
		}
		return leftName < rightName
	})
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
				if err := c.setEnv(k, value); err != nil {
					c.CleanEnv()
					return err
				}
			}
		}
		for k := range c.Configuration.Additional {
			if value, ok := configuration.Additional[k]; ok {
				if err := c.setEnv(k, value); err != nil {
					c.CleanEnv()
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
			c.restoreEnv(k)
		}
		for k := range c.Configuration.Additional {
			c.restoreEnv(k)
		}
	}
}

func normalizeEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 {
		return trimmed
	}

	first := trimmed[0]
	last := trimmed[len(trimmed)-1]
	if first != last || first != '"' && first != '\'' {
		return trimmed
	}
	if first == '\'' {
		return trimmed[1 : len(trimmed)-1]
	}

	unquoted, err := strconv.Unquote(trimmed)
	if err != nil {
		return trimmed[1 : len(trimmed)-1]
	}

	return unquoted
}

func (c *Config) setEnv(key, value string) error {
	c.backupEnv(key)
	return os.Setenv(key, normalizeEnvValue(value))
}

func (c *Config) backupEnv(key string) {
	if c.envBackup == nil {
		c.envBackup = make(map[string]*string)
	}
	if _, ok := c.envBackup[key]; ok {
		return
	}

	value, exists := os.LookupEnv(key)
	if !exists {
		c.envBackup[key] = nil
		return
	}

	copied := value
	c.envBackup[key] = &copied
}

func (c *Config) restoreEnv(key string) {
	if c.envBackup == nil {
		_ = os.Unsetenv(key)
		return
	}

	value, exists := c.envBackup[key]
	if !exists || value == nil {
		_ = os.Unsetenv(key)
		return
	}

	_ = os.Setenv(key, *value)
}
