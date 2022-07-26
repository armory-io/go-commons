package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubValues(t *testing.T) {
	m := map[string]interface{}{
		"mock": map[string]interface{}{
			"somekey": "${mock.flat.otherkey.value}",
			"flat": map[string]interface{}{
				"otherkey": map[string]interface{}{
					"value": "mockReplaceValue",
				},
			},
		},
	}

	subValues(m, m, nil)
	testValue := m["mock"].(map[string]interface{})["somekey"]
	assert.Equal(t, "mockReplaceValue", testValue)
}

func TestResolveSubs(t *testing.T) {
	m := map[string]interface{}{
		"mock": map[string]interface{}{
			"flat": map[string]interface{}{
				"otherkey": map[string]interface{}{
					"value": "mockValue",
				},
			},
		},
	}
	str := resolveSubs(m, "mock.flat.otherkey.value", nil)
	assert.Equal(t, "mockValue", str)
}
