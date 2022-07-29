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
//	type MyConfiguration struct {
//		prop1 string
//		boolProp bool
//		someList []string
//	}
//
// 	conf := typesafeconfig.Resolve[MyConfiguration](log,
//		typesafeconfig.BaseConfigurationNames("myappname"), // defaults to application
//		typesafeconfig.ActiveProfiles("prod"),
//	)
package typesafeconfig

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/armory-io/go-commons/secrets"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
)

var NoConfigurationSourcesProvided = errors.New("no configuration sources provide, you must provided at least 1 embed.FS or dir path")

type resolver struct {
	log                 *zap.SugaredLogger
	embeddedFilesystems []*embed.FS
	configurationDirs   []string
	baseNames           []string
	profiles            []string
}

type Option = func(resolver *resolver)

func EmbeddedFilesystems(embeddedFilesystems ...*embed.FS) Option {
	return func(resolver *resolver) {
		resolver.embeddedFilesystems = embeddedFilesystems
	}
}

func Directories(directories ...string) Option {
	return func(resolver *resolver) {
		resolver.configurationDirs = directories
	}
}

func ActiveProfiles(profiles ...string) Option {
	return func(resolver *resolver) {
		resolver.profiles = profiles
	}
}

func BaseConfigurationNames(baseNames ...string) Option {
	return func(resolver *resolver) {
		resolver.baseNames = baseNames
	}
}

func defaultResolver() *resolver {
	configurationDirs := []string{"/opt/go-application/config"}
	usr, err := user.Current()
	if err == nil {
		configurationDirs = append(configurationDirs, filepath.Join(usr.HomeDir, ".armory"))
	}

	profiles := strings.Split(os.Getenv("PROFILES_ACTIVE"), ",")

	return &resolver{
		baseNames:         []string{"application"},
		configurationDirs: configurationDirs,
		profiles:          profiles,
	}
}

// Resolve given the provided options resolves your configuration
func Resolve[T any](log *zap.SugaredLogger, options ...Option) (*T, error) {
	r := defaultResolver()
	for _, option := range options {
		option(r)
	}

	if len(r.embeddedFilesystems) == 0 && len(r.configurationDirs) == 0 {
		return nil, NoConfigurationSourcesProvided
	}

	candidates := getConfigurationFileCandidates(r.configurationDirs, r.baseNames, r.profiles)
	sources, err := loadFileBasedConfigurationSources(log, candidates, r.embeddedFilesystems)
	if err != nil {
		return nil, err
	}
	sources = append(sources, loadEnvironmentSources())
	untypedConfig := mergeSources(sources...)
	if err = resolveSecrets(untypedConfig); err != nil {
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
	err = decoder.Decode(untypedConfig)
	return typeSafeConfig, err
}

func loadEnvironmentSources() map[string]interface{} {
	config := make(map[string]interface{})
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

func setValue(config map[string]interface{}, key []string, value any) {
	if len(key) == 1 {
		config[key[0]] = value
		return
	}
	cur, remaining := pop(key)
	var nested map[string]interface{}
	if config[cur] == nil {
		nested = make(map[string]interface{})
	} else {
		curNested := config[cur]
		unboxed, ok := curNested.(map[string]interface{})
		if !ok {
			nested = make(map[string]interface{})
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
func mergeSources(sources ...map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for _, unNormalizedSource := range sources {
		source := normalizeKeys(unNormalizedSource)
		// iterate through key and if the value is a map recurse, else set the key to the value if type is a number, list or boolean
		for _, key := range maps.Keys(source) {
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
				typedCur := cur.(map[string]interface{})
				if valT.Kind() == reflect.Map {
					typedVal := val.(map[string]interface{})
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

func resolveSecrets(config map[string]interface{}) error {
	for _, key := range maps.Keys(config) {
		val := config[key]
		valT := reflect.TypeOf(val)
		if valT.Kind() == reflect.Map {
			if err := resolveSecrets(val.(map[string]interface{})); err != nil {
				return err
			}
		}
		if valT.Kind() == reflect.String && secrets.IsEncryptedSecret(val.(string)) {
			d, err := secrets.NewDecrypter(context.TODO(), val.(string))
			if err != nil {
				return err
			}
			plainTextValue, err := d.Decrypt()
			if err != nil {
				return err
			}
			config[key] = plainTextValue
		}
	}
	return nil
}

func normalizeKeys(source map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{})
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
) ([]map[string]interface{}, error) {
	var sources []map[string]interface{}
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

func loadCandidateFromEmbeddedFs(filesystem fs.FS, candidate string) (map[string]interface{}, error) {
	data, err := fs.ReadFile(filesystem, candidate)
	if err != nil {
		return nil, nil
	}
	return unmarshalData(data, candidate)
}

func unmarshalData(data []byte, candidate string) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, multierr.Append(
			fmt.Errorf("failed to unmarshel configuration: %s", candidate),
			err,
		)
	}
	return config, nil
}

func loadCandidate(candidate string) (map[string]interface{}, error) {
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
