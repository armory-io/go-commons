/*
 * Copyright (c) 2022 Armory, Inc
 *   National Electronics and Computer Technology Center, Thailand
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
