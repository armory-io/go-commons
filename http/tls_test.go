package http

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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
	err := CheckFileExists(fmt.Sprintf("encrypted:noop!asdf"))
	if !assert.NotNil(t, err) {
		return
	}
	assert.Equal(t, "no file referenced, use encryptedFile", err.Error())

	err = CheckFileExists(fmt.Sprintf("encryptedFile:noop!asdf"))
	if !assert.Nil(t, err) {
		return
	}

	// set up a non empty temp file
	tmpfile, err := ioutil.TempFile("", "cert")
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
