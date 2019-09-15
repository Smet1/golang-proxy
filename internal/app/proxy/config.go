package proxy

import (
	"errors"
	"time"
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
	Protocol    string      `yaml:"protocol"`
	Certificate Certificate `yaml:"certificate"`
	Timeout     Duration    `yaml:"timeout"`
	ServeAddr   string      `yaml:"serve_addr"`
}

type Certificate struct {
	Pem string `yaml:"pem"`
	Key string `yaml:"key"`
}

func (c *Config) Validate() error {
	if c.Protocol != "http" && c.Protocol != "https" {
		return errors.New("protocol must be either http or https")
	}

	return nil
}
