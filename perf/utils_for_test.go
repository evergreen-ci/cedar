package perf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mongodb/grip"
)

const defaultSeed = 12345678

func isEvergreen() bool { return os.Getenv("EVR_TASK_ID") != "" }

func LoadFixture(testName string, fixture interface{}) error {
	parts := strings.Split(testName, "/")
	testName = parts[len(parts)-1]

	fixtureName := fmt.Sprintf("testdata/%s.json", testName)
	jsonFile, err := os.Open(fixtureName)
	if err != nil {
		return err
	}
	defer func() { grip.Alert(jsonFile.Close()) }()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(byteValue, fixture)
}
