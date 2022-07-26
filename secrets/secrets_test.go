package secrets

import (
	"context"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	RegisterVaultConfig(VaultConfig{
		Enabled:    true,
		Url:        "http://mytest.url",
		AuthMethod: "TOKEN",
		Token:      "abcdef",
	})
	m.Run()
	delete(Engines, "vault")
}

func TestEngineCheck(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
		params   string
		isFile   bool
	}{
		{
			"Regular string no engine",
			"test",
			"",
			"",
			false,
		},
		{
			"Empty string",
			"",
			"",
			"",
			false,
		},
		{
			"s3 engine",
			"encrypted:s3!b:blah",
			"s3",
			"b:blah",
			false,
		},
		{
			"s3 engine as file",
			"encryptedFile:s3!b:blah",
			"s3",
			"b:blah",
			true,
		},
		{
			"even no params should recognize the engine",
			"encrypted:s3!",
			"s3",
			"",
			false,
		},
		{
			"unregistered engines are recognized",
			"encrypted:blah!",
			"blah",
			"",
			false,
		},
		{
			"but we need a bang separator",
			"encrypted:s3",
			"",
			"",
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, f, p := GetEngine(c.value)
			assert.Equal(t, c.expected, e)
			assert.Equal(t, c.isFile, f)
			assert.Equal(t, c.params, p)
		})
	}
}

func TestParseYaml(t *testing.T) {
	cases := []struct {
		key         string
		expected    string
		shouldError bool
	}{
		{
			"rootKey",
			"rootValue",
			false,
		},
		{
			"this.is.nested",
			"nestedValue",
			false,
		},
		{
			"doesnt.exist",
			"",
			true,
		},
	}
	for _, c := range cases {
		secretBytes := []byte("rootKey: rootValue\nthis:\n is:\n  nested: nestedValue")
		actual, err := parseSecretFile(secretBytes, c.key)
		didError := (err != nil)
		if actual != c.expected || didError != c.shouldError {
			t.Errorf("for parseSecretFile(%s) with error=='%t' -- wanted [%s], "+
				"but got [%s] with error=='%t", c.key, c.shouldError, c.expected, actual, didError)
		}
	}

	invalidYaml := []byte("this:\n is:invalid\n is: yaml")
	actual, err := parseSecretFile(invalidYaml, "this.is")
	if len(actual) > 0 || err == nil {
		t.Errorf("for parseSecretFile() with invalid yaml, expected error with empty return value but got "+
			"error: %q and return value: %q", err, actual)
	}
}

func TestParseS3Secret(t *testing.T) {
	cases := []struct {
		params      string
		expected    func(*testing.T, *S3Decrypter)
		shouldError bool
	}{
		{
			"r:region!b:bucket!f:file",
			func(t *testing.T, decrypter *S3Decrypter) {
				assert.Equal(t, "region", decrypter.region)
				assert.Equal(t, "bucket", decrypter.bucket)
				assert.Equal(t, "file", decrypter.filepath)
			},
			false,
		},
		{
			"r:region!b:bucket!f:file!k:key",
			func(t *testing.T, decrypter *S3Decrypter) {
				assert.Equal(t, "region", decrypter.region)
				assert.Equal(t, "bucket", decrypter.bucket)
				assert.Equal(t, "file", decrypter.filepath)
				assert.Equal(t, "key", decrypter.key)

			},
			false,
		},
		{
			"b:bucket!f:file",
			func(t *testing.T, decrypter *S3Decrypter) {},
			true,
		},
		{
			"r:region!f:file",
			func(t *testing.T, decrypter *S3Decrypter) {},
			true,
		},
		{
			"r:region!b:bucket",
			func(t *testing.T, decrypter *S3Decrypter) {},
			true,
		},
	}

	for _, c := range cases {
		s3 := &S3Decrypter{}
		err := s3.parse(c.params)
		if assert.Equal(t, c.shouldError, err != nil) {
			c.expected(t, s3)
		}
	}
}

func TestParseGcsSecret(t *testing.T) {
	cases := []struct {
		params      string
		expected    func(*testing.T, *GcsDecrypter)
		shouldError bool
	}{
		{
			"b:bucket!f:file",
			func(t *testing.T, decrypter *GcsDecrypter) {
				assert.Equal(t, "bucket", decrypter.bucket)
				assert.Equal(t, "file", decrypter.filepath)
			},
			false,
		},
		{
			"b:bucket!f:file!k:key",
			func(t *testing.T, decrypter *GcsDecrypter) {
				assert.Equal(t, "bucket", decrypter.bucket)
				assert.Equal(t, "file", decrypter.filepath)
				assert.Equal(t, "key", decrypter.key)
			},
			false,
		},
		{
			"b:bucket",
			func(t *testing.T, decrypter *GcsDecrypter) {},
			true,
		},
	}

	for _, c := range cases {
		gcs := &GcsDecrypter{}
		err := gcs.parse(c.params)
		if assert.Equal(t, c.shouldError, err != nil) {
			c.expected(t, gcs)
		}
	}
}

func TestDecrypter(t *testing.T) {
	cases := []struct {
		secretConfig string
		expected     interface{}
	}{
		{
			"encrypted:s3!b:bucket!r:us-west-2!f:file",
			&S3Decrypter{},
		},
		{
			"encrypted:gcs!b:bucket!f:file",
			&GcsDecrypter{},
		},
		{
			"encrypted:vault!e:engine!p:file!k:mykey",
			&VaultDecrypter{},
		},
		{
			"notASecret",
			&NoopDecrypter{},
		},
	}

	for _, c := range cases {
		decrypter, err := NewDecrypter(context.TODO(), c.secretConfig)
		assert.Nil(t, err)
		if reflect.TypeOf(decrypter) != reflect.TypeOf(c.expected) {
			t.Errorf("for parseS3EncryptedSecret(%s) -- expected type %s but got type %s",
				c.secretConfig, reflect.TypeOf(c.expected), reflect.TypeOf(decrypter))
		}
	}
}
