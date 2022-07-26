package secrets

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"testing"
)

type MockAwsSecretsManagerClient struct {
	mock.Mock
	t       *testing.T
	payload string
}

func (m *MockAwsSecretsManagerClient) FetchSecret(secretName string) (*secretsmanager.GetSecretValueOutput, error) {
	mockPayloadBytes, _ := ioutil.ReadFile("./aws-secrets-manager/" + m.payload)
	res := &secretsmanager.GetSecretValueOutput{}
	err := json.Unmarshal(mockPayloadBytes, res)
	if err != nil {
		m.t.Fatalf("could not unmarshal fixture JSON err: %v", err)
	}
	m.On("fetchSecret", mock.Anything).Return(res, nil)
	return res, nil
}

func TestNewAwsSecretsManagerDecrypter(t *testing.T) {
	cases := []struct {
		name          string
		params        string
		expectedError string
		isFile        bool
	}{
		{
			name:          "The provided params has less than the 2 required tokens",
			params:        "r:some-region",
			expectedError: GenericMalformedKeyError,
			isFile:        false,
		},
		{
			name:          "The provided params has a malformed kv pair",
			params:        "r:some-region!s!some-secret",
			expectedError: GenericMalformedKeyError,
			isFile:        false,
		},
		{
			name:          "The provided params is missing required param region",
			params:        "g:some-region!s:some-secret",
			expectedError: RegionMissingError,
			isFile:        false,
		},
		{
			name:          "The provided params is missing required param secret name",
			params:        "r:some-region!g:some-secret",
			expectedError: SecretNameMissingError,
			isFile:        false,
		},
		{
			name:          "An encrypted file specified a key param",
			params:        "r:some-region!s:some-secret!k:some-key",
			expectedError: EncryptedFilesShouldNotSpecifyKeyError,
			isFile:        true,
		},
		{
			name:          "extra unknown params where included",
			params:        "r:some-region!s:some-secret!k:some-key!q:some-unknown-thing",
			expectedError: GenericMalformedKeyError,
			isFile:        false,
		},
		{
			name:          "the happy path returns no error for file",
			params:        "r:some-region!s:some-secret",
			expectedError: "",
			isFile:        true,
		},
		{
			name:          "the happy path returns no error for plain text secret",
			params:        "r:some-region!s:some-secret",
			expectedError: "",
			isFile:        false,
		},
		{
			name:          "the happy path returns no error for kv pair secret",
			params:        "r:some-region!s:some-secret!k:some-key",
			expectedError: "",
			isFile:        false,
		},
		{
			name:          "secret references full secret ARN",
			params:        "r:some-region!s:my:secret/path/some-secret!k:some-key",
			expectedError: "",
			isFile:        false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sut, err := NewAwsSecretsManagerDecrypter(nil, c.isFile, c.params)
			var errorMsg string
			if err != nil {
				errorMsg = err.Error()
			} else {
				assert.Equal(t, c.isFile, sut.IsFile())
			}
			assert.Equal(t, c.expectedError, errorMsg)
		})
	}
}

func TestDecrypt(t *testing.T) {
	cases := []struct {
		name           string
		secretKey      string
		payload        string
		expectedError  string
		isFile         bool
		expectedFile   string
		expectedSecret string
	}{
		{
			name:           "The provided params is for a binary file, such as a .pfx PKCS cert, ca, key wrapper file",
			secretKey:      "some-secret",
			payload:        "binary.json",
			expectedError:  "",
			isFile:         true,
			expectedFile:   "example-cert-ca-key-bundle.pfx",
			expectedSecret: "",
		},
		{
			name:           "The provided params is for a plain text file, such as a ssh key",
			secretKey:      "some-secret",
			payload:        "ssh-key.json",
			expectedError:  "",
			isFile:         true,
			expectedFile:   "example-ssh-key.pem",
			expectedSecret: "",
		},
		{
			name:           "The provided params is for a kv map and a specific key",
			secretKey:      "foo",
			payload:        "kvpairs.json",
			expectedError:  "",
			isFile:         false,
			expectedFile:   "",
			expectedSecret: "bar",
		},
		{
			name:           "The provided params is for a single plaintext secret",
			secretKey:      "",
			payload:        "plaintext.json",
			expectedError:  "",
			isFile:         false,
			expectedFile:   "",
			expectedSecret: "value",
		},
		{
			name:           "The provided params is for a kv map and a specific key, but the configured secret is plain text",
			secretKey:      "some-secret",
			payload:        "plaintext.json",
			expectedError:  MalformedKVPairSecretPayload,
			isFile:         false,
			expectedFile:   "",
			expectedSecret: "",
		},
		{
			name:           "The provided params is for a kv map and a specific key, but the configured secret is binary",
			secretKey:      "some-secret",
			payload:        "binary.json",
			expectedError:  MalformedKVPairSecretPayload,
			isFile:         false,
			expectedFile:   "",
			expectedSecret: "",
		},
		{
			name:           "The provided params is for a kv map and a specific key, but the configured secrets value is an embedded object and not a string",
			secretKey:      "some-secret",
			payload:        "custom.json",
			expectedError:  MalformedKVPairSecretPayload,
			isFile:         false,
			expectedFile:   "",
			expectedSecret: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			awsSecretsManagerClient := &MockAwsSecretsManagerClient{payload: c.payload, t: t}
			sut := &AwsSecretsManagerDecrypter{isFile: c.isFile, region: "some-region", secretName: "some-secret", secretKey: c.secretKey, awsSecretsManagerClient: awsSecretsManagerClient}

			secret, err := sut.Decrypt()
			var errorMsg string
			if err != nil {
				errorMsg = err.Error()
			}
			assert.Equal(t, c.expectedError, errorMsg)

			if c.expectedError != "" {
				return
			}

			if c.isFile {
				expectedBytes, _ := ioutil.ReadFile("./aws-secrets-manager/" + c.expectedFile)
				actualBytes, _ := ioutil.ReadFile(secret)
				assert.Equal(t, expectedBytes, actualBytes)
			} else {
				assert.Equal(t, c.expectedSecret, secret)
			}
		})
	}
}
