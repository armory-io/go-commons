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
//	import . "github.com/armory-io/go-commons/typesafeconfig"
//
//	type MyConfiguration struct {
//		prop1 string
//		boolProp bool
//		someList []string
//	}
//
// 	conf := ResolveConfiguration[MyConfiguration](log,
//		WithBaseConfigurationNames("myappname"), // defaults to application
//		WithActiveProfiles("prod"),
//	)
package typesafeconfig

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/armory-io/go-commons/secrets"
	"github.com/cbroglie/mustache"
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
				resolver.explicitProperties = mergeSources(resolver.explicitProperties, pAny.(map[string]any))
			case string:
				kvPair := strings.SplitN(pAny.(string), "=", 2)
				rawKey := kvPair[0]
				value := kvPair[1]
				key := strings.Split(rawKey, ".")
				setValue(resolver.explicitProperties, key, value)
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
	untypedConfig := mergeSources(sources...)
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
		setValue(config, key, value)
	}
	return config
}

func setValue(config map[string]any, key []string, value any) {
	if len(key) == 1 {
		config[key[0]] = value
		return
	}
	cur, remaining := pop(key)
	var nested map[string]any
	if config[cur] == nil {
		nested = make(map[string]any)
	} else {
		curNested := config[cur]
		unboxed, ok := curNested.(map[string]any)
		if !ok {
			nested = make(map[string]any)
		} else {
			nested = unboxed
		}
	}
	config[cur] = nested
	setValue(nested, remaining, value)
}

func pop[T any](array []T) (T, []T) {
	return array[0], array[1:]
}

// mergeSources recursively left merges config sources, omitting any non-map values that are not one of: lists, numbers, or booleans
// un-flattens keys before merging into new map
func mergeSources(sources ...map[string]any) map[string]any {
	m := make(map[string]any)
	for _, unNormalizedSource := range sources {
		source := normalizeKeys(unNormalizedSource)
		// iterate through key and if the value is a map recurse, else set the key to the value if type is a number, list or boolean
		for key := range source {
			val := source[key]
			cur := m[key]
			if cur == nil {
				m[key] = val
				continue
			}

			curT := reflect.TypeOf(cur)
			valT := reflect.TypeOf(val)
			switch curT.Kind() {
			case reflect.Map:
				typedCur := cur.(map[string]any)
				if valT.Kind() == reflect.Map {
					typedVal := val.(map[string]any)
					m[key] = mergeSources(typedCur, typedVal)
				} else {
					m[key] = val
				}
			case reflect.Array, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
				m[key] = val
			}
		}
	}
	return m
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

func normalizeKeys(source map[string]any) map[string]any {
	m := make(map[string]any)
	// un-flatten keys, ['foo.bar.bam']=true -> ['foo']['bar']['bam']=true
	for _, key := range maps.Keys(source) {
		normalizedKey := strings.ToLower(key)
		val := source[key]
		if strings.Contains(normalizedKey, ".") {
			parts := strings.Split(normalizedKey, ".")
			setValue(m, parts, val)
		} else {
			m[normalizedKey] = val
		}
	}
	return m
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

			log.Infof("successfully loaded candidate: %s", candidate)
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
	for _, dir := range configurationDirs {
		for _, baseName := range baseNames {
			candidates = append(candidates,
				fmt.Sprintf("%s/%s.yaml", dir, baseName),
				fmt.Sprintf("%s/%s.yml", dir, baseName))
			for _, profile := range profiles {
				candidates = append(candidates,
					fmt.Sprintf("%s/%s-%s.yaml", dir, baseName, profile),
					fmt.Sprintf("%s/%s-%s.yml", dir, baseName, profile))
			}
		}
	}

	return candidates
}
