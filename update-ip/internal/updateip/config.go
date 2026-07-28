package updateip

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the conventional system-wide configuration location.
const DefaultConfigPath = "/etc/update-ip.yml"

// Config contains the YAML settings required to update one DNS record.
type Config struct {
	Token        string `yaml:"token"`
	Organization string `yaml:"organization"`
	Domain       string `yaml:"domain"`
	Subdomain    string `yaml:"subdomain"`
	TTL          int    `yaml:"ttl"`
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	return parseConfig(data)
}

func parseConfig(data []byte) (Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return config, config.validate()
}

// validate keeps the organization YAML-compatible as a string while enforcing the API's positive numeric ID.
func (c Config) validate() error {
	switch {
	case c.Token == "":
		return errors.New("token is required")
	case c.Organization == "":
		return errors.New("organization is required")
	case c.Domain == "":
		return errors.New("domain is required")
	case c.Subdomain == "":
		return errors.New("subdomain is required")
	}

	organization, err := strconv.ParseUint(c.Organization, 10, 64)
	if err != nil {
		return fmt.Errorf("organization must be a positive integer: %w", err)
	}
	if organization == 0 {
		return errors.New("organization must be a positive integer")
	}
	if c.TTL <= 0 {
		return errors.New("ttl must be a positive integer")
	}
	return nil
}
