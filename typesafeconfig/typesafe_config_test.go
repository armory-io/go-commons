/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package typesafeconfig

import (
	"embed"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"os"
	"testing"
)

//go:embed test_resources/*
var testResources embed.FS

type EmbeddedSubConfig struct {
	SomeOtherStringOption string
}
type Config struct {
	FeatureEnabled    bool
	NumberOfWidgets   int
	SomeStringOption  string
	SomeUnsetValue    string
	EmbeddedSubConfig EmbeddedSubConfig
}

type TypesafeConfigTestSuite struct {
	logicalTestResourcePath string
	log                     *zap.SugaredLogger
	suite.Suite
	VariableThatShouldStartAtFive int
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (s *TypesafeConfigTestSuite) SetupSuite() {
	logger, _ := zap.NewDevelopment()
	s.log = logger.Sugar()

	dname, err := os.MkdirTemp("", "typesafe-config-test")
	if err != nil {
		s.T().Fatal(err)
	}

	cp.Copy("./test_resources", dname)
	s.logicalTestResourcePath = dname
}

func (s *TypesafeConfigTestSuite) TearDownSuite() {
	os.RemoveAll(s.logicalTestResourcePath)
}

func (s *TypesafeConfigTestSuite) TestSetValue() {
	type kvPair struct {
		key   []string
		value string
	}
	type test struct {
		name     string
		kvPairs  []kvPair
		config   map[string]interface{}
		expected map[string]interface{}
	}

	tests := []test{
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
			config: make(map[string]interface{}),
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
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
			config: make(map[string]interface{}),
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
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
			config: make(map[string]interface{}),
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"bam": map[string]interface{}{
							"baz": "some-value",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		for _, kvPair := range tc.kvPairs {
			setValue(tc.config, kvPair.key, kvPair.value)
		}
		assert.Equal(s.T(), tc.expected, tc.config, tc.name)
	}
}

func (s *TypesafeConfigTestSuite) TestMergeSources() {
	m1 := map[string]interface{}{
		"some-number": 10,
		"some-book":   true,
		"foo": map[string]interface{}{
			"bar": map[string]interface{}{
				"bam":         "value",
				"override-me": "original-value",
			},
		},
		"mutate-me": map[string]interface{}{
			"wut": true,
		},
	}
	m2 := map[string]interface{}{
		"foo": map[string]interface{}{
			"some-other-bool": false,
			"bar": map[string]interface{}{
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

	m3 := map[string]interface{}{
		"some.flattened.nested.key": true,
	}

	expected := map[string]interface{}{
		"some-number": 10,
		"some-book":   true,
		"foo": map[string]interface{}{
			"some-other-bool": false,
			"bar": map[string]interface{}{
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
		"some": map[string]interface{}{
			"flattened": map[string]interface{}{
				"nested": map[string]interface{}{
					"key": true,
				},
			},
		},
	}
	newMap := mergeSources(m1, m2, m3)
	assert.Equal(s.T(), expected, newMap)
}

func (s *TypesafeConfigTestSuite) TestResolve() {
	type kvPair struct {
		key   string
		value string
	}
	type test struct {
		name     string
		expected *Config
		options  []Option
		envVars  []kvPair
	}

	tests := []test{
		{
			name: "test that resolve produces the expected config when using an embedded fs",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "this is another string",
				},
			},
			options: []Option{
				EmbeddedFilesystems(&testResources),
				BaseConfigurationNames("basic-config"),
				Directories("test_resources"),
			},
			envVars: []kvPair{
				{
					key:   "PROFILES_ACTIVE",
					value: "prod",
				},
			},
		},
		{
			name: "test that resolve produces the expected config when using an embedded fs and a profile",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "overridden",
				},
			},
			options: []Option{
				EmbeddedFilesystems(&testResources),
				BaseConfigurationNames("basic-config"),
				Directories("test_resources"),
				ActiveProfiles("profile1"),
			},
		},
		{
			name: "test that resolve produces the expected config when using an embedded fs and a profile and an env var override",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "this is a new string from the env var",
				},
			},
			options: []Option{
				EmbeddedFilesystems(&testResources),
				BaseConfigurationNames("basic-config"),
				Directories("test_resources"),
				ActiveProfiles("profile1"),
			},
			envVars: []kvPair{
				{
					key:   "EMBEDDEDSUBCONFIG_SOMEOTHERSTRINGOPTION",
					value: "this is a new string from the env var",
				},
			},
		},
		{
			name: "test that resolve produces the expected config when using a snake cased config",
			expected: &Config{
				FeatureEnabled: true,
			},
			options: []Option{
				EmbeddedFilesystems(&testResources),
				BaseConfigurationNames("snake-case"),
				Directories("test_resources"),
			},
		},
		{
			name: "test that resolve produces the expected config when using the file system",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "this is another string",
				},
			},
			options: []Option{
				BaseConfigurationNames("basic-config"),
				Directories(s.logicalTestResourcePath),
			},
		},
		{
			name: "test that resolve produces the expected config when using the file system and a secret value",
			expected: &Config{
				SomeStringOption: "v:the-value",
			},
			options: []Option{
				BaseConfigurationNames("config-with-secret"),
				Directories(s.logicalTestResourcePath),
			},
		},
	}

	for _, tc := range tests {
		for _, enVar := range tc.envVars {
			os.Setenv(enVar.key, enVar.value)
		}
		actual, _ := Resolve[Config](s.log, tc.options...)
		assert.Equal(s.T(), tc.expected, actual, tc.name)
		for _, enVar := range tc.envVars {
			os.Unsetenv(enVar.key)
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(TypesafeConfigTestSuite))
}
