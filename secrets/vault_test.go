package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockVaultClient struct {
	mock.Mock
	token         string
	writeResponse *api.Secret
	writeErr      error
	readErr       error
	readWarnings  []string
	readData      map[string]interface{}
}

func (m *MockVaultClient) Write(path string, data map[string]interface{}) (*api.Secret, error) {
	m.Called(path, data)
	return m.writeResponse, m.writeErr
}

func (m *MockVaultClient) Read(path string) (*api.Secret, error) {
	m.Called(path)
	return &api.Secret{
		Data:     m.readData,
		Warnings: m.readWarnings,
	}, nil
}

type versionedResponse struct {
	expectedPath string
	response     *api.Secret
	err          error
}
type fakeVaultClient struct {
	t          *testing.T
	v1response versionedResponse
	v2response versionedResponse
}

func (f *fakeVaultClient) Write(path string, data map[string]interface{}) (*api.Secret, error) {
	panic("implement me")
}

func (f *fakeVaultClient) Read(path string) (*api.Secret, error) {
	switch {
	case path == f.v1response.expectedPath:
		return f.v1response.response, f.v1response.err
	case path == f.v2response.expectedPath:
		return f.v2response.response, f.v2response.err
	default:
		f.t.Fatalf("invalid path called: %s", path)
		return nil, nil
	}
}

func TestVaultDecrypter_fetchSecret(t *testing.T) {
	engine := "t1"
	path := "t2"
	cases := map[string]struct {
		vc             *fakeVaultClient
		expectedSecret string
		expectedError  string
	}{
		"calling v1 path returns a secret": {
			expectedSecret: "testvalue",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"key": "testvalue",
						},
					},
					err: nil,
				},
			},
		},
		"v1 returns warning, v2 returns a secret": {
			expectedSecret: "testvalue",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     &api.Secret{Warnings: []string{"Invalid path for a versioned K/V secrets engine"}},
					err:          nil,
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"data": map[string]interface{}{
								"key": "testvalue",
							},
						},
					},
					err: nil,
				},
			},
		},
		"v1 returns error, v2 returns a secret": {
			expectedSecret: "testvalue",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     &api.Secret{},
					err:          fmt.Errorf("permission denied"),
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"data": map[string]interface{}{
								"key": "testvalue",
							},
						},
					},
					err: nil,
				},
			},
		},
		"v1 returns empty response, v2 returns a secret": {
			expectedSecret: "testvalue",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     nil,
					err:          nil,
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"data": map[string]interface{}{
								"key": "testvalue",
							},
						},
					},
					err: nil,
				},
			},
		},
		"v1 returns connection error": {
			expectedError: "check connection to the server",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     nil,
					err:          &json.SyntaxError{},
				},
			},
		},
		"v1 returns non-connection error, v2 returns error": {
			expectedError: "error fetching secret from vault",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     nil,
					err:          fmt.Errorf("some other error"),
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response:     nil,
					err:          fmt.Errorf("some other error"),
				},
			},
		},
		"v2 returns error": {
			expectedError: "error fetching secret from vault",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     &api.Secret{Warnings: []string{"Invalid path for a versioned K/V secrets engine"}},
					err: nil,
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response:     nil,
					err:          fmt.Errorf("some other error"),
				},
			},
		},
		"v2 returns empty response": {
			expectedError: "couldn't find vault path",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     &api.Secret{Warnings: []string{"Invalid path for a versioned K/V secrets engine"}},
					err: nil,
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response:     nil,
					err:          nil,
				},
			},
		},
		"secret key not found at v1 path": {
			expectedError: "not found at engine",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"incorrect-key": "testvalue",
						},
					},
					err: nil,
				},
			},
		},
		"secret key not found at v2 path": {
			expectedError: "not found at engine",
			vc: &fakeVaultClient{
				v1response: versionedResponse{
					expectedPath: engine + "/" + path,
					response:     &api.Secret{Warnings: []string{"Invalid path for a versioned K/V secrets engine"}},
					err: nil,
				},
				v2response: versionedResponse{
					expectedPath: engine + "/data/" + path,
					response: &api.Secret{
						Data: map[string]interface{}{
							"data": map[string]interface{}{
								"incorrect-key": "testvalue",
							},
						},
					},
					err: nil,
				},
			},
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			c.vc.t = t
			d := &VaultDecrypter{path: path, engine: engine, key: "key"}
			s, err := d.fetchSecret(c.vc)
			if err != nil {
				if c.expectedError != "" {
					assert.Contains(t, err.Error(), c.expectedError)
				} else {
					t.Fatal(err)
				}
			}
			assert.Equal(t, c.expectedSecret, s)
		})
	}
}

func TestUserPassAuth(t *testing.T) {
	cases := map[string]struct {
		config       VaultConfig
		client       *MockVaultClient
		expectedPath string
		expectedData map[string]interface{}
		expectedError string
	}{
		"happy path": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "password",
				UserAuthPath: "path",
			},
			client: &MockVaultClient{
				token: "my-token",
				writeResponse: &api.Secret{
					Auth: &api.SecretAuth{
						ClientToken: "my-token",
					},
				},
			},
			expectedPath: "auth/path/login/user",
			expectedData: map[string]interface{}{"password": "password"},
			expectedError:  "",
		},
		"error from client gets returned": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "password",
				UserAuthPath: "path",
			},
			client: &MockVaultClient{
				token:    "",
				writeErr: errors.New("some error"),
			},
			expectedPath: "auth/path/login/user",
			expectedData: map[string]interface{}{"password": "password"},
			expectedError:  "error logging into vault:",
		},
		"connection error from client gets caught": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "password",
				UserAuthPath: "path",
			},
			client: &MockVaultClient{
				token:    "",
				writeErr: &json.SyntaxError{},
			},
			expectedPath: "auth/path/login/user",
			expectedData: map[string]interface{}{"password": "password"},
			expectedError:  "check connection to the server",
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			c.client.On("Write", c.expectedPath, c.expectedData).Return(c.client.writeResponse, c.client.writeErr)

			d := &VaultDecrypter{
				vaultConfig: c.config,
			}
			d.setTokenFetcher()
			token, err := d.tokenFetcher.fetchToken(c.client)
			assert.Equal(t, c.client.token, token)
			if c.expectedError != "" {
				assert.True(t, strings.Contains(err.Error(), c.expectedError))
			} else {
				assert.Nil(t, err)
			}

			// assert Write() method called with expected arguments
			c.client.AssertExpectations(t)
		})
	}
}

var mockJwt = "mock-k8s-token"

func mockFileReader(filename string) ([]byte, error) {
	return []byte(mockJwt), nil
}

func TestKubernetesAuth(t *testing.T) {
	cases := map[string]struct {
		config        VaultConfig
		client        *MockVaultClient
		expectedPath  string
		expectedData  map[string]interface{}
		expectedToken string
		expectedError   string
	}{
		"happy path": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "kubernetes",
			},
			client: &MockVaultClient{
				token: "my-initial-token",
				writeResponse: &api.Secret{
					Auth: &api.SecretAuth{
						ClientToken: "vault-token",
					},
				},
			},
			expectedPath: "auth/kubernetes/login",
			expectedData: map[string]interface{}{
				"role": "role",
				"jwt":  mockJwt,
			},
			expectedToken: "vault-token",
			expectedError: "",
		},
		"error from client gets returned": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "kubernetes",
			},
			client: &MockVaultClient{
				token:    "",
				writeErr: errors.New("some error"),
			},
			expectedPath: "auth/kubernetes/login",
			expectedData: map[string]interface{}{
				"role": "role",
				"jwt":  mockJwt,
			},
			expectedError: "error logging into vault:",
		},
		"connection error gets caught": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "kubernetes",
			},
			client: &MockVaultClient{
				token:    "",
				writeErr: &json.SyntaxError{},
			},
			expectedPath: "auth/kubernetes/login",
			expectedData: map[string]interface{}{
				"role": "role",
				"jwt":  mockJwt,
			},
			expectedError:  "check connection to the server",
		},

	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			c.client.On("Write", c.expectedPath, c.expectedData).Return(c.client.writeResponse, c.client.writeErr)

			tokenFetcher := KubernetesServiceAccountTokenFetcher{
				role:       c.config.Role,
				path:       c.config.Path,
				fileReader: mockFileReader,
			}
			token, err := tokenFetcher.fetchToken(c.client)
			assert.Equal(t, c.expectedToken, token)
			if c.expectedError != "" {
				assert.True(t, strings.Contains(err.Error(), c.expectedError))
			} else {
				assert.Nil(t, err)
			}
			// assert Write() method called with expected arguments
			c.client.AssertExpectations(t)
		})
	}
}

func TestTokenAuth(t *testing.T) {
	cases := map[string]struct {
		expectedToken string
		expectError   bool
	}{
		"happy path": {
			expectedToken: "mock-env-token",
			expectError:   false,
		},
		"env variable not set": {
			expectedToken: "",
			expectError:   true,
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			os.Setenv("VAULT_TOKEN", c.expectedToken)
			tokenFetcher := EnvironmentVariableTokenFetcher{}
			token, err := tokenFetcher.fetchToken(nil)
			assert.Equal(t, c.expectError, err != nil)
			assert.Equal(t, c.expectedToken, token)
			os.Unsetenv("VAULT_TOKEN")

		})
	}
}

func TestValidateVaultConfig(t *testing.T) {
	cases := map[string]struct {
		config  VaultConfig
		wantErr bool
	}{
		"vault not enabled": {
			config: VaultConfig{
				Enabled:    false,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "path",
			},
			wantErr: true,
		},
		"empty config": {
			config: VaultConfig{},
			wantErr: true,
		},
		"missing URL": {
			config: VaultConfig{
				Enabled:    true,
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "path"},
			wantErr: true,
		},
		"missing auth method": {
			config: VaultConfig{
				Enabled: true,
				Url:     "vault.com",
			},
			wantErr: true,
		},
		"token auth missing env variable": {
			config: VaultConfig{
				Enabled:    true,
				AuthMethod: "TOKEN",
			},
			wantErr: true,
		},
		"kubernetes auth happy path": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "path",
			},
			wantErr: false,
		},
		"kubernetes auth missing role": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "",
				Path:       "path",
			},
			wantErr: true,
		},
		"kubernetes auth missing path": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "KUBERNETES",
				Role:       "role",
				Path:       "",
			},
			wantErr: true,
		},
		"userpass auth happy path": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "password",
				UserAuthPath: "path",
			},
			wantErr: false,
		},
		"userpass auth missing user": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "",
				Password:     "password",
				UserAuthPath: "path",
			},
			wantErr: true,
		},
		"userpass auth missing password": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "",
				UserAuthPath: "path",
			},
			wantErr: true,
		},
		"userpass auth missing userAuthPath": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "user",
				Password:     "password",
				UserAuthPath: "",
			},
			wantErr: true,
		},
		"token auth missing env var": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "TOKEN",
			},
			wantErr: true,
		},
		"unknown auth method returns error": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "UNKNOWN",
			},
			wantErr: true,
		},
		"if Token already set, we don't need to look in env var": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "TOKEN",
				Role:       "",
				Path:       "",
				Token:      "s.123123123",
			},
			wantErr: false,
		},
	}

	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			if err := validateVaultConfig(c.config); (err != nil) != c.wantErr {
				t.Errorf("validateVaultConfig() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestSetTokenFetcher(t *testing.T) {
	cases := map[string]struct {
		config               VaultConfig
		expectedTokenFetcher TokenFetcher
		expectError          bool
	}{
		"token auth": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "TOKEN",
			},
			expectedTokenFetcher: EnvironmentVariableTokenFetcher{},
			expectError:          false,
		},
		"userpass auth": {
			config: VaultConfig{
				Enabled:      true,
				Url:          "vault.com",
				AuthMethod:   "USERPASS",
				Username:     "my-user",
				Password:     "my-password",
				UserAuthPath: "my-path",
			},
			expectedTokenFetcher: UserPassTokenFetcher{
				username:     "my-user",
				password:     "my-password",
				userAuthPath: "my-path",
			},
			expectError: false,
		},
		"unknown auth": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "UNKNOWN",
			},
			expectError: true,
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {

			decrypter := &VaultDecrypter{
				vaultConfig: c.config,
			}
			err := decrypter.setTokenFetcher()

			assert.EqualValues(t, c.expectedTokenFetcher, decrypter.tokenFetcher)
			assert.True(t, c.expectError == (err != nil))
		})
	}
}

func TestKubernetesTokenFetcher(t *testing.T) {
	decrypter := &VaultDecrypter{
		vaultConfig: VaultConfig{
			Enabled:    true,
			Url:        "vault.com",
			AuthMethod: "KUBERNETES",
			Role:       "my-role",
			Path:       "my-path",
		},
	}
	err := decrypter.setTokenFetcher()
	assert.Nil(t, err)

	actualTokenFetcher, ok := decrypter.tokenFetcher.(KubernetesServiceAccountTokenFetcher)
	assert.True(t, ok)
	assert.Equal(t, "my-role", actualTokenFetcher.role)
	assert.Equal(t, "my-path", actualTokenFetcher.path)
}

func TestSetToken(t *testing.T) {
	cases := map[string]struct {
		config        VaultConfig
		expectedToken string
		expectError   bool
	}{
		"happy path": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "TOKEN",
			},
			expectedToken: "mock-token",
			expectError:   false,
		},
		"error fetching token": {
			config: VaultConfig{
				Enabled:    true,
				Url:        "vault.com",
				AuthMethod: "TOKEN",
			},
			expectedToken: "",
			expectError:   true,
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			os.Setenv("VAULT_TOKEN", c.expectedToken)
			decrypter := &VaultDecrypter{
				vaultConfig: c.config,
			}
			err := decrypter.setTokenFetcher()
			assert.Nil(t, err)

			assert.Equal(t, "", decrypter.vaultConfig.Token)

			err = decrypter.setToken()
			assert.Equal(t, c.expectError, err != nil)
			assert.Equal(t, c.expectedToken, decrypter.vaultConfig.Token)
			os.Unsetenv("VAULT_TOKEN")
		})
	}
}

func TestNoSecret(t *testing.T) {
	notASecret := "notASecret"
	eng, err := NewDecrypter(context.TODO(), notASecret)
	assert.Nil(t, err)

	unchanged, _ := eng.Decrypt()
	assert.Equal(t, notASecret, unchanged)
}

func TestNoVaultConfig(t *testing.T) {
	e := Engines["vault"]
	delete(Engines, "vault")
	decrypter, err := NewDecrypter(context.TODO(), "encrypted:vault!e:secret!n:test-secret!k:foo")
	assert.NotNil(t, err)
	assert.Nil(t, decrypter)
	Engines["vault"] = e
}

func TestNewClient(t *testing.T) {
	cases := map[string]struct {
		cfg VaultConfig
	}{
		"with namespace": {
			cfg: VaultConfig{
				Enabled:    true,
				Url:        "http://127.0.0.1:8200",
				AuthMethod: "KUBERNETES",
				Namespace:  "my-namespace",
				Role:       "my-role",
				Path:       "my-path",
				Token:      "my-token",
			},
		},
		"no namespace": {
			cfg: VaultConfig{
				Enabled:    true,
				Url:        "http://127.0.0.1:8200",
				AuthMethod: "KUBERNETES",
				Namespace:  "",
				Role:       "my-role",
				Path:       "my-path",
				Token:      "my-other-token",
			},
		},
		"no token": {
			cfg: VaultConfig{
				Enabled:    true,
				Url:        "http://127.0.0.1:8200",
				AuthMethod: "KUBERNETES",
				Role:       "my-role",
				Path:       "my-path",
				Token:      "",
			},
		},
	}

	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			d := VaultDecrypter{
				vaultConfig: c.cfg,
			}

			client, err := d.newAPIClient()
			assert.Nil(t, err)
			assert.Equal(t, c.cfg.Token, client.Token())
			assert.Equal(t, c.cfg.Url, client.Address())

			if c.cfg.Namespace != "" {
				headers := client.Headers()
				assert.NotNil(t, headers)
				assert.Equal(t, c.cfg.Namespace, headers.Get(consts.NamespaceHeaderName))
			}
		})
	}
}

func TestParseVaultSecret(t *testing.T) {
	cases := map[string]struct {
		params            string
		expectedDecrypter *VaultDecrypter
		shouldError       bool
	}{
		"standard syntax": {
			params: "e:engine!p:path!k:key",
			expectedDecrypter: &VaultDecrypter{
				engine: "engine",
				path:   "path",
				key:    "key",
			},
			shouldError: false,
		},
		"with deprecated n param": {
			params: "e:engine!n:path!k:key",
			expectedDecrypter: &VaultDecrypter{
				engine: "engine",
				path:   "path",
				key:    "key",
			},
			shouldError: false,
		},
		"with binary file flag": {
			params: "e:engine!p:path!k:key!b:true",
			expectedDecrypter: &VaultDecrypter{
				engine:        "engine",
				path:          "path",
				key:           "key",
				base64Encoded: "true",
			},
			shouldError: false,
		},
		"missing engine": {
			params: "n:path!k:key",
			expectedDecrypter: &VaultDecrypter{
				path: "path",
				key:  "key",
			},
			shouldError: true,
		},
		"missing path": {
			params: "e:engine!k:key",
			expectedDecrypter: &VaultDecrypter{
				engine: "engine",
				key:    "key",
			},
			shouldError: true,
		},
		"missing key": {
			params: "e:engine!p:path",
			expectedDecrypter: &VaultDecrypter{
				engine: "engine",
				path:   "path",
			},
			shouldError: true,
		},
	}

	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			v := &VaultDecrypter{}
			err := v.parseSyntax(c.params)
			assert.Equal(t, c.shouldError, err != nil)
			assert.EqualValues(t, c.expectedDecrypter, v)
		})
	}
}

func TestRetryableError(t *testing.T) {
	cases := map[string]struct {
		secret        *api.Secret
		err error
		expectedResult   bool
	}{
		"permission denied error returns true": {
			secret: nil,
			err: errors.New("permission denied"),
			expectedResult: true,
		},
		"nil mapping returns true": {
			secret: nil,
			expectedResult: true,
		},
		"invalid path warning returns true": {
			secret: &api.Secret{
				Warnings: []string{"Invalid path for a versioned K/V secrets engine"},
			},
			expectedResult: true,
		},
		"nil warning returns false": {
			secret: &api.Secret{
				Warnings: nil,
			},
			expectedResult: false,
		},
		"empty slice warning returns false": {
			secret: &api.Secret{
				Warnings: []string{},
			},
			expectedResult: false,
		},
	}
	for testName, c := range cases {
		t.Run(testName, func(t *testing.T) {
			actual := containsRetryableError(c.err, c.secret)
			assert.Equal(t, c.expectedResult, actual)
		})
	}
}



