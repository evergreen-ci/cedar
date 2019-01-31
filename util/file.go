package util

import (
	"io/ioutil"
	"os"

	"github.com/mongodb/grip"
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

func WriteFile(path string, data string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "problem creating file '%s'", path)
	}

	_, err = file.WriteString(data)
	if err != nil {
		grip.Warning(errors.Wrapf(file.Close(), "problem closing file '%s' after error", path))
		return errors.Wrapf(err, "problem writing data to file '%s'", path)
	}

	if err = file.Close(); err != nil {
		return errors.Wrapf(err, "problem closing file '%s' after successfully writing data", path)
	}

	return nil
}
