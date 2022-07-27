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

package config

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	yamlParse "gopkg.in/yaml.v2"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

type Loader struct {
	DefaultConfigDirs []string
	DefaultProfiles   []string
	ConfigDir         string
	EnvMap            map[string]string
}

func (c *Loader) initialize() {
	c.buildConfigDirs()
	c.DefaultProfiles = []string{"armory", "local"}
	c.ConfigDir = c.configDirectory()
	c.EnvMap = keyPairToMap(os.Environ())
}

func (c *Loader) buildConfigDirs() {
	var paths []string
	usr, err := user.Current()
	if err == nil {
		paths = append(paths, filepath.Join(usr.HomeDir, ".armory"))
	}
	c.DefaultConfigDirs = paths
}

func (c *Loader) configDirectory() string {
	for _, dir := range c.DefaultConfigDirs {
		if _, err := fs.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
}

func (c *Loader) profiles() []string {
	p := os.Getenv("PROFILES_ACTIVE")
	if len(p) > 0 {
		return strings.Split(p, ",")
	}
	return c.DefaultProfiles
}

// Use afero to create an abstraction layer between our package and the
// OS's file system. This will allow us to test our package.
var fs = afero.NewOsFs()

func loadConfig(configFile string) (map[interface{}]interface{}, error) {
	s := map[interface{}]interface{}{}
	if _, err := fs.Stat(configFile); err == nil {
		bytes, err := afero.ReadFile(fs, configFile)
		if err != nil {
			log.Errorf("Unable to open config file %s: %v", configFile, err)
			return nil, nil
		}
		if err = yamlParse.Unmarshal(bytes, &s); err != nil {
			return s, fmt.Errorf("unable to parse config file %s: %w", configFile, err)
		}
		log.Info("Configured with settings from file: ", configFile)
	} else {
		logFsStatError(err, "Config file ", configFile, " not present; falling back to default settings")
	}
	return s, nil
}

func logFsStatError(err error, args ...interface{}) {
	if os.IsNotExist(err) {
		log.WithError(err).Debug(args...)
		return
	}
	log.WithError(err).Error(args...)
}

func LoadDefault(propNames []string) (map[string]interface{}, error) {
	env := Loader{}
	env.initialize()
	return LoadDefaultWithEnv(env, propNames)
}

func LoadDefaultWithEnv(env Loader, propNames []string) (map[string]interface{}, error) {
	if env.ConfigDir == "" {
		return nil, errors.New("could not find config directory")
	}
	config, _, err := loadProperties(propNames, env.ConfigDir, env.profiles(), env.EnvMap)
	return config, err
}

func keyPairToMap(keyPairs []string) map[string]string {
	m := map[string]string{}
	for _, keyPair := range keyPairs {
		split := strings.Split(keyPair, "=")
		m[split[0]] = split[1]
	}
	return m
}

func loadProperties(propNames []string, confDir string, profiles []string, envMap map[string]string) (map[string]interface{}, []string, error) {
	var propMaps []map[interface{}]interface{}
	var filePaths []string
	//first load the main props, i.e. gate.yml/yaml with no profile extensions
	for _, prop := range propNames {
		// yaml is "official"
		config, filePath, err := loadPropertyFromFile(fmt.Sprintf("%s/%s", confDir, prop))
		// file might have been unparsable
		if err != nil {
			return nil, filePaths, err
		}
		if len(config) > 0 {
			propMaps = append(propMaps, config)
			filePaths = append(filePaths, filePath)
		}
	}

	for _, prop := range propNames {
		//we traverse the profiles array backwards for correct precedence
		//for i := len(profiles) - 1; i >= 0; i-- {
		for i := range profiles {
			p := profiles[i]
			pTrim := strings.TrimSpace(p)
			config, filePath, err := loadPropertyFromFile(fmt.Sprintf("%s/%s-%s", confDir, prop, pTrim))
			if err != nil {
				return nil, filePaths, err
			}
			if len(config) > 0 {
				propMaps = append(propMaps, config)
				filePaths = append(filePaths, filePath)
			}
		}
	}
	m, err := Resolve(propMaps, envMap)
	return m, filePaths, err
}

func loadPropertyFromFile(pathPrefix string) (map[interface{}]interface{}, string, error) {
	filePath := fmt.Sprintf("%s.yaml", pathPrefix)
	config, err := loadConfig(filePath)
	if err != nil {
		return config, filePath, err
	}
	if len(config) > 0 {
		return config, filePath, nil
	}

	// but people also use "yml" too, if we don't get anything let's try this
	filePath = fmt.Sprintf("%s.yml", pathPrefix)
	config, err = loadConfig(filePath)
	return config, filePath, err
}

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string { return &v }
