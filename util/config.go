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
		Host      string
		SshPort   int    `yaml:"sshPort"`
		HttpPort  int    `yaml:"httpPort"`
		SslDomain string `yaml:"sslDomain"`
		WithAp    bool   `yaml:"withAp"`
		Single    bool   `yaml:"single"`
		Closed    bool   `yaml:"closed"`
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

	if envHost != "" {
		c.Conf.Host = envHost
	}

	if envSshPort != "" {
		v, err := strconv.Atoi(envSshPort)
		if err != nil {
			fmt.Println(err)
		}
		c.Conf.SshPort = v
	}

	if envHttpPort != "" {
		v, err := strconv.Atoi(envHttpPort)
		if err != nil {
			fmt.Println(err)
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

	return c, nil
}
