package util

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
)

const Name = "stegodon"
const ConfigFileName = "config.yaml"

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

	buf, err := os.ReadFile(ConfigFileName)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", ConfigFileName, err)
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

	return c, err
}
