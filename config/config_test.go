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
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestConfigDirs(t *testing.T) {
	env := Loader{}
	env.initialize()
	assert.Equal(t, len(env.DefaultConfigDirs), 1)
}

func Test_logFsStatError(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir, err := afero.TempDir(fs, "/tmp", "testfstaterror")
	if !assert.NoError(t, err) {
		return
	}
	defer fs.RemoveAll(tempDir)

	previousOut := logrus.StandardLogger().Out
	defer logrus.SetOutput(previousOut)
	var buf bytes.Buffer
	logrus.SetOutput(io.MultiWriter(os.Stderr, &buf))

	a := afero.NewBasePathFs(fs, tempDir)
	_, err = a.Stat(".missingfile__")
	if !assert.True(t, os.IsNotExist(err)) {
		return
	}
	logFsStatError(err, "")
	if !assert.Len(t, buf.String(), 0) {
		return
	}

	dir := "test"
	file := "test/test"
	err = a.Mkdir(dir, 0755)
	if !assert.NoError(t, err) {
		return
	}
	f, err := a.Create(file)
	if !assert.NoError(t, err) {
		return
	}
	_ = f.Close()

	err = a.Chmod(dir, 0222)
	if !assert.NoError(t, err) {
		return
	}

	_, err = a.Stat(file)
	if !assert.Error(t, err) {
		return
	}
	logFsStatError(err, "")
	if !assert.Contains(t, buf.String(), "level=error") {
		return
	}
}

func TestLoadProperties(t *testing.T) {
	prevfs := fs
	defer func() { fs = prevfs }()
	fs = afero.NewMemMapFs()
	configFile := "/kubesvc.yaml"
	content := `
kubernetes:
  accounts:
    -name: gke_github-replication-sandbox_us-central1-c_kubesvc-testing1-dev
      kubeconfigFile: /kubeconfigfiles/kubeconfig
`
	var err error
	var file afero.File
	if file, err = fs.Create(configFile); !assert.NoError(t, err) {
		return
	}
	if _, err := file.WriteString(content); !assert.NoError(t, err) {
		return
	}

	// Test
	config, paths, err := loadProperties([]string{"kubesvc"}, "", []string{}, map[string]string{})

	const expectedMessage = "unable to parse config file"
	if !assert.Len(t, paths, 0) {
		return
	}
	if !assert.Len(t, config, 0) {
		return
	}
	if !assert.Error(t, err, "with message %q", expectedMessage) {
		return
	}
	if !assert.Contains(t, err.Error(), expectedMessage) {
		return
	}
}

func TestEnvProfiles(t *testing.T) {
	_ = os.Setenv("PROFILES_ACTIVE", "profile-1,profile-2")
	env := Loader{}
	assert.ElementsMatchf(t, []string{"profile-1", "profile-2"}, env.profiles(), "")
}
