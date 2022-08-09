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

package mysql

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/yaml"
	"testing"
	"time"
)

type Temp struct {
	Dur MDuration `yaml:"dur"`
}

func TestDuration(t *testing.T) {
	cases := []struct {
		name        string
		ser         string
		expectedVal time.Duration
	}{
		{
			"default",
			"",
			0 * time.Minute,
		},
		{
			"10 minutes",
			"dur: 10m",
			10 * time.Minute,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t2 *testing.T) {
			tmp := &Temp{}

			if err := yaml.Unmarshal([]byte(c.ser), tmp); err != nil {
				t2.Fatal("unable to parse input string")
			}
			assert.Equal(t2, c.expectedVal, tmp.Dur.Duration)
		})
	}
}

func TestDuration_Err(t *testing.T) {
	tmp := &Temp{}
	err := yaml.Unmarshal([]byte("dur: not_a_duration"), tmp)
	assert.NotNil(t, err)
}

func TestDatabase_ConnectionUrl(t *testing.T) {
	set := Configuration{
		Connection:      "net(localhost:3006)/test",
		User:            "root",
		Password:        "mypassword",
		MigrateUser:     "migrateuser",
		MigratePassword: "migratepwd",
	}
	s, err := set.ConnectionUrl(false)
	assert.Nil(t, err)
	assert.Equal(t, "root:mypassword@net(localhost:3006)/test?parseTime=true", s)

	s, err = set.ConnectionUrl(true)
	assert.Nil(t, err)
	assert.Equal(t, "mysql://migrateuser:migratepwd@net(localhost:3006)/test", s)
}

func TestDatabase_ConnectionUrl2(t *testing.T) {
	set := Configuration{
		Connection: "that_is_not_a_connection_string",
	}
	_, err := set.ConnectionUrl(false)
	assert.NotNil(t, err)
}
