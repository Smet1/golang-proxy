package proxy

import (
	"time"

	"github.com/pkg/errors"
)

const (
	HTTP  = "http"
	HTTPS = "https"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringDuration := ""
	err := unmarshal(&stringDuration)
	if err != nil {
		return err
	}

	d.Duration, err = time.ParseDuration(stringDuration)
	return err
}

type Config struct {
	Protocol       string      `yaml:"protocol"`
	Certificate    Certificate `yaml:"certificate"`
	Timeout        Duration    `yaml:"timeout"`
	ServeAddrProxy string      `yaml:"serve_addr_proxy"`
	ServeAddrBurst string      `yaml:"serve_addr_burst"`
	DB             DB          `yaml:"db"`
}

type DB struct {
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"ssl_mode"`
	Host     string `yaml:"host"`
}

type Certificate struct {
	Pem string `yaml:"pem"`
	Key string `yaml:"key"`
}

func (c *Config) Validate() error {
	if c.Protocol != HTTP && c.Protocol != HTTPS {
		return errors.New("protocol must be either http or https")
	}

	return nil
}
