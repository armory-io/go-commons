package metrics

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestMetrics(t *testing.T) {

	assert.NoError(t, os.Setenv("HOSTNAME", "foo"))
	defer os.Unsetenv("HOSTNAME")

	s := &Settings{
		Environment:     "muh-environment",
		Version:         "v1.0.0",
		ApplicationName: "deploy-engine",
	}

	tags := getBaseTags(*s)
	assert.Equal(t, tags["appName"], "deploy-engine")
	assert.Equal(t, tags["version"], "v1.0.0")
	assert.Equal(t, tags["hostname"], "foo")
	assert.Equal(t, tags["environment"], "muh-environment")
	assert.Equal(t, tags["replicaset"], "UNKNOWN")
}
