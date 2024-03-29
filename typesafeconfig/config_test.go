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
	List              []string
	ListOptions       []ChildOption
	EmbeddedSubConfig EmbeddedSubConfig
}

type ChildOption struct {
	Name  string
	Value string
}

type TypesafeConfigTestSuite struct {
	logicalTestResourcePath string
	log                     *zap.SugaredLogger
	suite.Suite
	VariableThatShouldStartAtFive int
}

type kvPair struct {
	key   string
	value string
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

func (s *TypesafeConfigTestSuite) TestAdditionDirs() {
	r := &resolver{
		log:               s.log,
		configurationDirs: []string{"foo"},
	}
	WithAdditionalDirectories("bar")(r)
	assert.Equal(s.T(), []string{"foo", "bar"}, r.configurationDirs)
}

func (s *TypesafeConfigTestSuite) TestResolve() {

	tests := []struct {
		name     string
		expected *Config
		options  []Option
		envVars  []kvPair
	}{
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
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("basic-config"),
				WithDirectories("test_resources"),
			},
			envVars: []kvPair{
				{
					key:   "ADDITIONAL_ACTIVE_PROFILES",
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
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("basic-config"),
				WithDirectories("test_resources"),
				WithActiveProfiles("profile1"),
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
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("basic-config"),
				WithDirectories("test_resources"),
				WithActiveProfiles("profile1"),
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
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("snake-case"),
				WithDirectories("test_resources"),
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
				WithBaseConfigurationNames("basic-config"),
				WithDirectories(s.logicalTestResourcePath),
			},
		},
		{
			name: "test that resolve produces the expected config when using the file system and a secret value",
			expected: &Config{
				SomeStringOption: "v:the-value",
				ListOptions: []ChildOption{
					{
						Name:  "option1",
						Value: "v:first-value",
					},
					{
						Name:  "option2",
						Value: "v:second-value",
					},
					{
						Name:  "option3",
						Value: "v:third-value",
					},
				},
			},
			options: []Option{
				WithBaseConfigurationNames("config-with-secret"),
				WithDirectories(s.logicalTestResourcePath),
			},
		},
		{
			name: "test that resolve produces the expected config when an explicit property",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "there can only be one",
				},
			},
			options: []Option{
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("basic-config"),
				WithDirectories("test_resources"),
				WithActiveProfiles("profile1"),
				WithExplicitProperties(
					"embeddedSubConfig.someOtherStringOption=there can only be one",
				),
			},
			envVars: []kvPair{
				{
					key:   "EMBEDDEDSUBCONFIG_SOMEOTHERSTRINGOPTION",
					value: "this is a new string from the env var",
				},
			},
		},
		{
			name: "test that resolve produces the expected config when an explicit property map",
			expected: &Config{
				FeatureEnabled:   true,
				NumberOfWidgets:  10,
				SomeStringOption: "this is a string",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "this is a new string from the env var",
				},
				List: []string{
					"item1",
					"item2",
				},
			},
			options: []Option{
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("basic-config"),
				WithDirectories("test_resources"),
				WithActiveProfiles("profile1"),
				WithExplicitProperties(
					map[string]any{
						"list": []string{
							"item1",
							"item2",
						},
					},
				),
			},
			envVars: []kvPair{
				{
					key:   "EMBEDDEDSUBCONFIG_SOMEOTHERSTRINGOPTION",
					value: "this is a new string from the env var",
				},
			},
		},
		{
			name: "test that resolve produces the expected config with an env var reference",
			expected: &Config{
				FeatureEnabled:   false,
				NumberOfWidgets:  5,
				SomeStringOption: "some-env-var-value",
				EmbeddedSubConfig: EmbeddedSubConfig{
					SomeOtherStringOption: "this is another string",
				},
			},
			options: []Option{
				WithEmbeddedFilesystems(&testResources),
				WithBaseConfigurationNames("config-with-templates"),
				WithDirectories("test_resources"),
			},
			envVars: []kvPair{
				{
					key:   "SOME_ENV_VAR",
					value: "some-env-var-value",
				},
			},
		},
	}

	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			for _, enVar := range tc.envVars {
				os.Setenv(enVar.key, enVar.value)
			}
			actual, err := ResolveConfiguration[Config](s.log, tc.options...)
			if err != nil {
				t.Fatalf(err.Error())
			}
			assert.Equal(t, tc.expected, actual)
			for _, enVar := range tc.envVars {
				os.Unsetenv(enVar.key)
			}
		})
	}
}

func (s *TypesafeConfigTestSuite) TestGetConfigurationFileCandidates() {
	tests := []struct {
		name              string
		expected          []string
		configurationDirs []string
		baseNames         []string
		profiles          []string
		envVars           []kvPair
	}{
		{
			name:              "test that getConfigurationFileCandidates applies ADDITIONAL_ACTIVE_PROFILES last and that profiles are applied in order when there are multiple dirs",
			configurationDirs: []string{"/foo", "/bar"},
			baseNames:         []string{"my-app"},
			profiles:          []string{"prod"},
			envVars: []kvPair{
				{
					key:   "ADDITIONAL_ACTIVE_PROFILES",
					value: "prod-overrides",
				},
			},
			expected: []string{
				"/foo/my-app.yaml",
				"/foo/my-app.yml",
				"/bar/my-app.yaml",
				"/bar/my-app.yml",
				"/foo/my-app-prod.yaml",
				"/foo/my-app-prod.yml",
				"/bar/my-app-prod.yaml",
				"/bar/my-app-prod.yml",
				"/foo/my-app-prod-overrides.yaml",
				"/foo/my-app-prod-overrides.yml",
				"/bar/my-app-prod-overrides.yaml",
				"/bar/my-app-prod-overrides.yml",
			},
		},
	}
	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			for _, enVar := range tc.envVars {
				os.Setenv(enVar.key, enVar.value)
			}
			actual := getConfigurationFileCandidates(tc.configurationDirs, tc.baseNames, tc.profiles)
			assert.Equal(t, tc.expected, actual)
			for _, enVar := range tc.envVars {
				os.Unsetenv(enVar.key)
			}
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestTypesafeConfigTestSuite(t *testing.T) {
	suite.Run(t, new(TypesafeConfigTestSuite))
}
