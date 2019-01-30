package util

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

func ReadFileYAML(path string, target interface{}) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.Errorf("file %s does not exist", path)
	}

	yamlData, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "invalid file: %s", path)
	}

	if err := yaml.Unmarshal(yamlData, target); err != nil {
		return errors.Wrapf(err, "problem parsing yaml/json from file %s", path)
	}

	return nil
}

func FileExists(path string) bool {
	if path == "" {
		return false
	}

	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}
