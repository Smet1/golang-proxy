package configreader

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func ReadConfig(fileName string, config interface{}) error {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrap(err, "cant read config file")
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return errors.Wrap(err, "cant parse config")
	}

	return nil
}
