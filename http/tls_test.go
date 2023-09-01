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

package http

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestEncryptedKey(t *testing.T) {
	cases := []struct {
		name        string
		password    string
		errExpected bool
		expected    string
	}{
		{
			name:        "not encrypted",
			password:    "asdf1234",
			errExpected: false,
			expected:    "asdf1234",
		},
		{
			name:        "encrypted noop",
			password:    "encrypted:noop!asdf1234",
			errExpected: false,
			expected:    "asdf1234",
		},
		{
			name:        "encrypted fail",
			password:    "encrypted:doesnotexist!asdf1234",
			errExpected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := getKeyPassword(c.password)
			if assert.Equal(t, c.errExpected, err != nil) {
				assert.Equal(t, c.expected, p)
			}
		})
	}
}

func TestFileReadable(t *testing.T) {
	err := CheckFileExists("encrypted:noop!asdf")
	if !assert.NotNil(t, err) {
		return
	}
	assert.Equal(t, "no file referenced, use encryptedFile", err.Error())

	err = CheckFileExists("encryptedFile:noop!asdf")
	if !assert.Nil(t, err) {
		return
	}

	// set up a non empty temp file
	tmpfile, err := os.CreateTemp("", "cert")
	if !assert.Nil(t, err) {
		return
	}
	defer tmpfile.Close()

	// File should be readable
	assert.Nil(t, CheckFileExists(tmpfile.Name()))

	// Remove the file
	os.Remove(tmpfile.Name())
	assert.NotNil(t, CheckFileExists(tmpfile.Name()))
}
