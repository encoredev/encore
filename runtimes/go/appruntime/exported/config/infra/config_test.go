package infra

import (
	"encoding/json"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestInfraConfigMarshalUnmarshal(t *testing.T) {
	c := qt.New(t)
	// Read the test data file
	data, err := os.ReadFile("testdata/infra.config.json")
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON data into InfraConfig
	var config InfraConfig
	err = json.Unmarshal(data, &config)
	c.Assert(err, qt.IsNil)

	// Marshal the InfraConfig back to JSON
	marshaledData, err := json.Marshal(config)
	c.Assert(err, qt.IsNil)

	// Unmarshal the marshaled JSON data back to InfraConfig and compare the two objects
	var newConfig InfraConfig
	err = json.Unmarshal(marshaledData, &newConfig)
	c.Assert(err, qt.IsNil)

	c.Assert(newConfig, qt.DeepEquals, config)
}
