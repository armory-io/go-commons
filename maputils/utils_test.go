package maputils

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
)

type MapUtilsTestSuite struct {
	log *zap.SugaredLogger
	suite.Suite
}

func (m *MapUtilsTestSuite) SetupSuite() {
	logger, _ := zap.NewDevelopment()
	m.log = logger.Sugar()
}

func (m *MapUtilsTestSuite) TestSetValue() {
	type kvPair struct {
		key   []string
		value string
	}

	tests := []struct {
		name     string
		kvPairs  []kvPair
		config   map[string]any
		expected map[string]any
	}{
		{
			name: "test that a nested key can be set into a new map",
			kvPairs: []kvPair{
				{
					key:   []string{"foo", "bar", "bam"},
					value: "baz",
				},
				{
					key:   []string{"foo", "bar", "bop"},
					value: "wow",
				},
			},
			config: make(map[string]any),
			expected: map[string]any{
				"foo": map[string]any{
					"bar": map[string]any{
						"bam": "baz",
						"bop": "wow",
					},
				},
			},
		},
		{
			name: "test that values can be overridden",
			kvPairs: []kvPair{
				{
					key:   []string{"foo", "bar", "bam"},
					value: "baz",
				},
				{
					key:   []string{"foo", "bar", "bam"},
					value: "overridden",
				},
			},
			config: make(map[string]any),
			expected: map[string]any{
				"foo": map[string]any{
					"bar": map[string]any{
						"bam": "overridden",
					},
				},
			},
		},
		{
			name: "test that a key that has a value is overridden by proceeding nested config",
			kvPairs: []kvPair{
				{
					key:   []string{"foo", "bar", "bam"},
					value: "value1",
				},
				{
					key:   []string{"foo", "bar", "bam", "baz"},
					value: "some-value",
				},
			},
			config: make(map[string]any),
			expected: map[string]any{
				"foo": map[string]any{
					"bar": map[string]any{
						"bam": map[string]any{
							"baz": "some-value",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		m.T().Run(tc.name, func(t *testing.T) {
			for _, kvPair := range tc.kvPairs {
				SetValue(tc.config, kvPair.key, kvPair.value)
			}
			assert.Equal(m.T(), tc.expected, tc.config)
		})
	}
}

func (m *MapUtilsTestSuite) TestMergeSources() {
	m1 := map[string]any{
		"some-number": 10,
		"some-book":   true,
		"foo": map[string]any{
			"bar": map[string]any{
				"bam":         "value",
				"override-me": "original-value",
			},
		},
		"mutate-me": map[string]any{
			"wut": true,
		},
	}
	m2 := map[string]any{
		"foo": map[string]any{
			"some-other-bool": false,
			"bar": map[string]any{
				"bop": "wow",
				"fiz": []string{
					"foo",
					"bar",
				},
				"override-me": "new-value",
			},
		},
		"mutate-me": false,
	}

	m3 := map[string]any{
		"some.flattened.nested.key": true,
	}

	expected := map[string]any{
		"some-number": 10,
		"some-book":   true,
		"foo": map[string]any{
			"some-other-bool": false,
			"bar": map[string]any{
				"bam":         "value",
				"bop":         "wow",
				"override-me": "new-value",
				"fiz": []string{
					"foo",
					"bar",
				},
			},
		},
		"mutate-me": false,
		"some": map[string]any{
			"flattened": map[string]any{
				"nested": map[string]any{
					"key": true,
				},
			},
		},
	}
	newMap := MergeSources(m1, m2, m3)
	assert.Equal(m.T(), expected, newMap)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMapUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(MapUtilsTestSuite))
}
