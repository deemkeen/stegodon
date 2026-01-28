package util

import (
	_ "embed"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strconv"
)

const Name = "stegodon"
const ConfigFileName = "config.yaml"

//go:embed config_default.yaml
var embeddedConfig []byte

type AppConfig struct {
	Conf struct {
		Host            string
		SshPort         int    `yaml:"sshPort"`
		HttpPort        int    `yaml:"httpPort"`
		SslDomain       string `yaml:"sslDomain"`
		WithAp          bool   `yaml:"withAp"`
		Single          bool   `yaml:"single"`
		Closed          bool   `yaml:"closed"`
		NodeDescription string `yaml:"nodeDescription"`
		WithJournald    bool   `yaml:"withJournald"`
		WithPprof       bool   `yaml:"withPprof"`
		MaxChars        int    `yaml:"maxChars"`
		ShowGlobal      bool   `yaml:"showGlobal"`
		SshOnly         bool   `yaml:"sshOnly"`
		ShowTos         bool   `yaml:"showTos"`
	}
}

func ReadConf() (*AppConfig, error) {

	c := &AppConfig{}

	// Try to resolve config file path (local first, then user dir)
	configPath := ResolveFilePath(ConfigFileName)

	var buf []byte
	var err error

	// Try to read from resolved path
	buf, err = os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, use embedded config and create user config file
		log.Printf("Config file not found at %s, using embedded defaults", configPath)
		buf = embeddedConfig

		// Try to write default config to user config directory
		configDir, dirErr := GetConfigDir()
		if dirErr == nil {
			userConfigPath := configDir + "/" + ConfigFileName
			writeErr := os.WriteFile(userConfigPath, embeddedConfig, 0644)
			if writeErr != nil {
				log.Printf("Warning: could not write default config to %s: %v", userConfigPath, writeErr)
			} else {
				log.Printf("Created default config file at %s", userConfigPath)
			}
		}
	}

	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in config file: %w", err)
	}

	envHost := os.Getenv("STEGODON_HOST")
	envSshPort := os.Getenv("STEGODON_SSHPORT")
	envHttpPort := os.Getenv("STEGODON_HTTPPORT")
	envSslDomain := os.Getenv("STEGODON_SSLDOMAIN")
	envWithAp := os.Getenv("STEGODON_WITH_AP")
	envSingle := os.Getenv("STEGODON_SINGLE")
	envClosed := os.Getenv("STEGODON_CLOSED")
	envNodeDescription := os.Getenv("STEGODON_NODE_DESCRIPTION")
	envWithJournald := os.Getenv("STEGODON_WITH_JOURNALD")
	envWithPprof := os.Getenv("STEGODON_WITH_PPROF")
	envMaxChars := os.Getenv("STEGODON_MAX_CHARS")
	envShowGlobal := os.Getenv("STEGODON_SHOW_GLOBAL")
	envSshOnly := os.Getenv("STEGODON_SSH_ONLY")
	envShowTos := os.Getenv("STEGODON_SHOW_TOS")

	if envHost != "" {
		c.Conf.Host = envHost
	}

	if envSshPort != "" {
		v, err := strconv.Atoi(envSshPort)
		if err != nil {
			log.Printf("Error parsing STEGODON_SSHPORT: %v", err)
		}
		c.Conf.SshPort = v
	}

	if envHttpPort != "" {
		v, err := strconv.Atoi(envHttpPort)
		if err != nil {
			log.Printf("Error parsing STEGODON_HTTPPORT: %v", err)
		}
		c.Conf.HttpPort = v
	}

	if envSslDomain != "" {
		c.Conf.SslDomain = envSslDomain
	}

	if envWithAp == "true" {
		c.Conf.WithAp = true
	}

	if envSingle == "true" {
		c.Conf.Single = true
	}

	if envClosed == "true" {
		c.Conf.Closed = true
	}

	if envNodeDescription != "" {
		c.Conf.NodeDescription = envNodeDescription
	}

	if envWithJournald == "true" {
		c.Conf.WithJournald = true
	}

	if envWithPprof == "true" {
		c.Conf.WithPprof = true
	}

	if envShowGlobal == "true" {
		c.Conf.ShowGlobal = true
	}

	if envSshOnly == "true" {
		c.Conf.SshOnly = true
	}

	if envShowTos == "true" {
		c.Conf.ShowTos = true
	}

	if envMaxChars != "" {
		v, err := strconv.Atoi(envMaxChars)
		if err != nil {
			log.Printf("Error parsing STEGODON_MAX_CHARS: %v", err)
		} else {
			// Apply maximum limit of 300 characters
			if v > 300 {
				log.Printf("STEGODON_MAX_CHARS value %d exceeds maximum of 300, capping at 300", v)
				c.Conf.MaxChars = 300
				// Catch less then 1 character in config.
			} else if v < 1 {
				log.Printf("STEGODON_MAX_CHARS value %d is less than minimum of 1, setting to default 150", v)
				c.Conf.MaxChars = 150
			} else {
				c.Conf.MaxChars = v
			}
		}
	}

	// Set default value if not set in config or environment
	if c.Conf.MaxChars == 0 {
		c.Conf.MaxChars = 150
	} else if c.Conf.MaxChars > 300 {
		// Apply maximum limit of 300 characters for config file values too
		log.Printf("maxChars value %d in config exceeds maximum of 300, capping at 300", c.Conf.MaxChars)
		c.Conf.MaxChars = 300
	} else if c.Conf.MaxChars < 1 {
		log.Printf("maxChars value %d in config is less than minimum of 1, setting to default 150", c.Conf.MaxChars)
		c.Conf.MaxChars = 150
	}

	return c, nil
}
