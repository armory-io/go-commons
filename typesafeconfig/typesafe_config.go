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

// Package typesafeconfig is for resolving configurations from many sources into a typesafe object
//
// Quickstart:
//
//	import . "github.com/armory-io/go-commons/typesafeconfig"
//
//	type MyConfiguration struct {
//		prop1 string
//		boolProp bool
//		someList []string
//	}
//
//	conf := ResolveConfiguration[MyConfiguration](log,
//		WithBaseConfigurationNames("myappname"), // defaults to application
//		WithActiveProfiles("prod"),
//	)
package typesafeconfig

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/armory-io/go-commons/maputils"
	"github.com/armory-io/go-commons/secrets"
	"github.com/cbroglie/mustache"
	"github.com/fatih/color"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"io/fs"
	"k8s.io/utils/strings/slices"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
)

var ErrNoConfigurationSourcesProvided = errors.New("no configuration sources provided, you must provide at least 1 embed.FS or dir path")

type resolver struct {
	log                 *zap.SugaredLogger
	embeddedFilesystems []*embed.FS
	configurationDirs   []string
	baseNames           []string
	profiles            []string
	explicitProperties  map[string]any
}

type Option = func(resolver *resolver)

func WithEmbeddedFilesystems(embeddedFilesystems ...*embed.FS) Option {
	return func(resolver *resolver) {
		resolver.embeddedFilesystems = embeddedFilesystems
	}
}

func WithDirectories(directories ...string) Option {
	return func(resolver *resolver) {
		resolver.configurationDirs = directories
	}
}

func WithAdditionalDirectories(directories ...string) Option {
	return func(resolver *resolver) {
		resolver.configurationDirs = append(resolver.configurationDirs, directories...)
	}
}

func WithActiveProfiles(profiles ...string) Option {
	return func(resolver *resolver) {
		resolver.profiles = profiles
	}
}

func WithBaseConfigurationNames(baseNames ...string) Option {
	return func(resolver *resolver) {
		resolver.baseNames = baseNames
	}
}

func WithExplicitProperties[T string | map[string]any](properties ...T) Option {
	return func(resolver *resolver) {
		for _, propertySource := range properties {
			pAny := any(propertySource)
			switch pAny.(type) {
			case map[string]any:
				resolver.explicitProperties = maputils.MergeSources(resolver.explicitProperties, pAny.(map[string]any))
			case string:
				kvPair := strings.SplitN(pAny.(string), "=", 2)
				rawKey := kvPair[0]
				value := kvPair[1]
				key := strings.Split(rawKey, ".")
				maputils.SetValue(resolver.explicitProperties, key, value)
			}
		}
	}
}

func defaultResolver() *resolver {
	configurationDirs := []string{"/opt/go-application/config", "resources"}
	usr, err := user.Current()
	if err == nil {
		configurationDirs = append(configurationDirs, filepath.Join(usr.HomeDir, ".armory"))
	}

	return &resolver{
		baseNames:          []string{"application"},
		configurationDirs:  configurationDirs,
		profiles:           []string{},
		explicitProperties: make(map[string]any),
	}
}

// ResolveConfiguration given the provided options resolves your configuration
func ResolveConfiguration[T any](log *zap.SugaredLogger, options ...Option) (*T, error) {
	r := defaultResolver()
	for _, option := range options {
		option(r)
	}

	if len(r.embeddedFilesystems) == 0 && len(r.configurationDirs) == 0 {
		return nil, ErrNoConfigurationSourcesProvided
	}

	candidates := getConfigurationFileCandidates(r.configurationDirs, r.baseNames, r.profiles)
	sources, err := loadFileBasedConfigurationSources(log, candidates, r.embeddedFilesystems)
	if err != nil {
		return nil, err
	}
	sources = append(sources,
		loadEnvironmentSources(),
		r.explicitProperties, // explicit properties should be the last source
	)
	untypedConfig := maputils.MergeSources(sources...)
	// hydrate secret tokens
	if err = resolveSecrets(untypedConfig); err != nil {
		return nil, err
	}
	// hydrate template tokens
	if err = resolveTemplates(untypedConfig); err != nil {
		return nil, err
	}
	var typeSafeConfig *T
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &typeSafeConfig,
		MatchName: func(mapKey, fieldName string) bool {
			normalizedMapKey := strings.ToLower(mapKey)
			normalizedMapKey = strings.ReplaceAll(normalizedMapKey, "-", "")
			normalizedMapKey = strings.ReplaceAll(normalizedMapKey, "_", "")
			return strings.ToLower(fieldName) == normalizedMapKey
		},
	})
	if err != nil {
		return nil, err
	}
	return typeSafeConfig, decoder.Decode(untypedConfig)
}

func loadEnvironmentSources() map[string]any {
	config := make(map[string]any)
	env := os.Environ()
	for _, envVar := range env {
		kvPair := strings.SplitN(envVar, "=", 2)
		rawKey := kvPair[0]
		value := kvPair[1]
		key := strings.Split(rawKey, "_")
		maputils.SetValue(config, key, value)
	}
	return config
}

// resolveTemplates resolves values that are mustache templates, but currently only sets the context to { "env": { [key: string]: string } }
func resolveTemplates(config map[string]any) error {
	envVars := make(map[string]string)
	env := os.Environ()
	for _, envVar := range env {
		kvPair := strings.SplitN(envVar, "=", 2)
		key := kvPair[0]
		value := kvPair[1]
		envVars[key] = value
	}

	templateContext := map[string]any{
		"env": envVars,
	}

	return recurseStringValuesAndMap(config, func(value string) (string, error) {
		parsedTemplate, err := mustache.ParseString(value)
		if err != nil {
			return value, err
		}
		renderedValue, err := parsedTemplate.Render(templateContext)
		if err != nil {
			return value, err
		}
		return renderedValue, nil
	})
}

func resolveSecrets(config map[string]any) error {
	return recurseStringValuesAndMap(config, func(value string) (string, error) {
		if secrets.IsEncryptedSecret(value) {
			d, err := secrets.NewDecrypter(context.Background(), value)
			if err != nil {
				return value, err
			}
			plainTextValue, err := d.Decrypt()
			if err != nil {
				return value, err
			}
			return plainTextValue, nil
		}
		return value, nil
	})
}

func recurseStringValuesAndMap(config map[string]any, valueMapper func(value string) (string, error)) error {
	for _, key := range maps.Keys(config) {
		val := config[key]
		valT := reflect.TypeOf(val)
		if valT.Kind() == reflect.Map {
			if err := recurseStringValuesAndMap(val.(map[string]any), valueMapper); err != nil {
				return err
			}
		}
		if valT.Kind() == reflect.String {
			value, err := valueMapper(val.(string))
			if err != nil {
				return err
			}
			config[key] = value
		}
	}
	return nil
}

func loadFileBasedConfigurationSources(
	log *zap.SugaredLogger,
	candidates []string,
	embeddedFilesystems []*embed.FS,
) ([]map[string]any, error) {
	var sources []map[string]any
	for _, candidate := range candidates {
		candidateFound := false
		// Scan through the list of embedded filesystems, stopping at the first found
		for _, filesystem := range embeddedFilesystems {
			config, err := loadCandidateFromEmbeddedFs(filesystem, candidate)
			if err != nil {
				return nil, err
			}

			if config == nil {
				continue
			}

			log.Infof(color.New(color.FgHiGreen, color.Bold).Sprintf("successfully loaded config source: %s", color.New(color.Underline).Sprintf(candidate)))
			sources = append(sources, config)
			candidateFound = true
			break
		}
		// If we don't find the candidate in an embed fs, scan the local fs
		if !candidateFound {
			config, err := loadCandidate(candidate)
			if err != nil {
				return nil, err
			}
			if config != nil {
				log.Infof("successfully loaded candidate: %s", candidate)
				sources = append(sources, config)
			}
		}
	}
	return sources, nil
}

func loadCandidateFromEmbeddedFs(filesystem fs.FS, candidate string) (map[string]any, error) {
	data, err := fs.ReadFile(filesystem, candidate)
	if err != nil {
		return nil, nil
	}
	return unmarshalData(data, candidate)
}

func unmarshalData(data []byte, candidate string) (map[string]any, error) {
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, multierr.Append(
			fmt.Errorf("failed to unmarshal configuration: %s", candidate),
			err,
		)
	}
	return config, nil
}

func loadCandidate(candidate string) (map[string]any, error) {
	data, err := os.ReadFile(candidate)
	if err != nil {
		return nil, nil
	}
	return unmarshalData(data, candidate)
}

func getConfigurationFileCandidates(
	configurationDirs []string,
	baseNames []string,
	profiles []string,
) []string {
	envVarSetProfiles := strings.Split(os.Getenv("ADDITIONAL_ACTIVE_PROFILES"), ",")
	for _, profile := range envVarSetProfiles {
		if !slices.Contains(profiles, profile) {
			profiles = append(profiles, profile)
		}
	}
	var candidates []string
	for _, baseName := range baseNames {
		for _, dir := range configurationDirs {
			candidates = append(candidates,
				fmt.Sprintf("%s/%s.yaml", dir, baseName),
				fmt.Sprintf("%s/%s.yml", dir, baseName))
		}
		for _, profile := range profiles {
			for _, dir := range configurationDirs {
				candidates = append(candidates,
					fmt.Sprintf("%s/%s-%s.yaml", dir, baseName, profile),
					fmt.Sprintf("%s/%s-%s.yml", dir, baseName, profile))
			}
		}
	}

	return candidates
}
