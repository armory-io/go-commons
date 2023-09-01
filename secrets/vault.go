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

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

type VaultConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Url          string `json:"url" yaml:"url"`
	AuthMethod   string `json:"authMethod" yaml:"authMethod"`
	Role         string `json:"role" yaml:"role"`
	Path         string `json:"path" yaml:"path"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	UserAuthPath string `json:"userAuthPath" yaml:"userAuthPath"`
	Namespace    string `json:"namespace" yaml:"namespace"`
	Token        string // no struct tags for token
}

type VaultSecret struct {
}

type VaultDecrypter struct {
	engine        string
	path          string
	key           string
	base64Encoded string
	isFile        bool
	vaultConfig   VaultConfig
	tokenFetcher  TokenFetcher
}

type VaultClient interface {
	Write(path string, data map[string]interface{}) (*api.Secret, error)
	Read(path string) (*api.Secret, error)
}

func RegisterVaultConfig(vaultConfig VaultConfig) error {
	if err := validateVaultConfig(vaultConfig); err != nil {
		return fmt.Errorf("vault configuration error - %s", err)
	}

	Engines["vault"] = func(ctx context.Context, isFile bool, params string) (Decrypter, error) {
		vd := &VaultDecrypter{isFile: isFile, vaultConfig: vaultConfig}
		if err := vd.parseSyntax(params); err != nil {
			return nil, err
		}
		if err := vd.setTokenFetcher(); err != nil {
			return nil, err
		}
		return vd, nil
	}
	return nil
}

type TokenFetcher interface {
	fetchToken(client VaultClient) (string, error)
}

type EnvironmentVariableTokenFetcher struct{}

func (e EnvironmentVariableTokenFetcher) fetchToken(client VaultClient) (string, error) {
	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		return "", fmt.Errorf("VAULT_TOKEN environment variable not set")
	}
	return token, nil
}

type UserPassTokenFetcher struct {
	username     string
	password     string
	userAuthPath string
}

func (u UserPassTokenFetcher) fetchToken(client VaultClient) (string, error) {
	data := map[string]interface{}{
		"password": u.password,
	}
	loginPath := "auth/" + u.userAuthPath + "/login/" + u.username

	log.Infof("logging into vault with USERPASS auth at: %s", loginPath)
	secret, err := client.Write(loginPath, data)
	if err != nil {
		return handleLoginErrors(err)
	}

	return secret.Auth.ClientToken, nil
}

type KubernetesServiceAccountTokenFetcher struct {
	role       string
	path       string
	fileReader fileReader
}

// define a file reader function so we can test kubernetes auth
type fileReader func(string) ([]byte, error)

func (k KubernetesServiceAccountTokenFetcher) fetchToken(client VaultClient) (string, error) {
	tokenBytes, err := k.fileReader("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", fmt.Errorf("error reading service account token: %s", err)
	}
	data := map[string]interface{}{
		"role": k.role,
		"jwt":  string(tokenBytes),
	}
	loginPath := "auth/" + k.path + "/login"

	log.Infof("logging into vault with KUBERNETES auth at: %s", loginPath)
	secret, err := client.Write(loginPath, data)
	if err != nil {
		return handleLoginErrors(err)
	}

	return secret.Auth.ClientToken, nil
}

func handleLoginErrors(err error) (string, error) {
	if _, ok := err.(*json.SyntaxError); ok {
		// some connection errors aren't properly caught, and the vault client tries to parse <nil>
		return "", fmt.Errorf("error fetching secret from vault - check connection to the server")
	}
	return "", fmt.Errorf("error logging into vault: %s", err)
}

func (v *VaultDecrypter) setTokenFetcher() error {
	var tokenFetcher TokenFetcher

	switch v.vaultConfig.AuthMethod {
	case "TOKEN":
		tokenFetcher = EnvironmentVariableTokenFetcher{}
	case "KUBERNETES":
		tokenFetcher = KubernetesServiceAccountTokenFetcher{
			role:       v.vaultConfig.Role,
			path:       v.vaultConfig.Path,
			fileReader: os.ReadFile,
		}
	case "USERPASS":
		tokenFetcher = UserPassTokenFetcher{
			username:     v.vaultConfig.Username,
			password:     v.vaultConfig.Password,
			userAuthPath: v.vaultConfig.UserAuthPath,
		}
	default:
		return fmt.Errorf("unknown Vault secrets auth method: %q", v.vaultConfig.AuthMethod)
	}

	v.tokenFetcher = tokenFetcher
	return nil
}

func (v *VaultDecrypter) Decrypt() (string, error) {
	if v.vaultConfig.Token == "" {
		err := v.setToken()
		if err != nil {
			return "", err
		}
	}
	client, err := v.getVaultClient()
	if err != nil {
		return "", err
	}
	secret, err := v.fetchSecret(client)
	if err != nil && strings.Contains(err.Error(), "403") {
		// get new token and retry in case our saved token is no longer valid
		if err := v.setToken(); err != nil {
			return "", err
		}
		secret, err = v.fetchSecret(client)
		if err != nil {
			return "", err
		}
	}
	if err != nil {
		return "", err
	}
	if v.IsFile() {
		return ToTempFile([]byte(secret))
	}
	return secret, nil
}

func (v *VaultDecrypter) IsFile() bool {
	return v.isFile
}

func (v *VaultDecrypter) parseSyntax(params string) error {
	tokens := strings.Split(params, "!")
	for _, element := range tokens {
		kv := strings.Split(element, ":")
		if len(kv) == 2 {
			switch kv[0] {
			case "e":
				v.engine = kv[1]
			case "p", "n":
				v.path = kv[1]
			case "k":
				v.key = kv[1]
			case "b":
				v.base64Encoded = kv[1]
			}
		}
	}

	if v.engine == "" {
		return fmt.Errorf("secret format error - 'e' for engine is required")
	}
	if v.path == "" {
		return fmt.Errorf("secret format error - 'p' for path is required (replaces deprecated 'n' param)")
	}
	if v.key == "" {
		return fmt.Errorf("secret format error - 'k' for key is required")
	}
	return nil
}

func validateVaultConfig(vaultConfig VaultConfig) error {
	if (VaultConfig{}) == vaultConfig {
		return fmt.Errorf("vault secrets not configured in service profile yaml")
	}
	if !vaultConfig.Enabled {
		return fmt.Errorf("vault secrets disabled")
	}
	if vaultConfig.Url == "" {
		return fmt.Errorf("vault url required")
	}
	if vaultConfig.AuthMethod == "" {
		return fmt.Errorf("auth method required")
	}

	switch vaultConfig.AuthMethod {
	case "TOKEN":
		if vaultConfig.Token == "" {
			envToken := os.Getenv("VAULT_TOKEN")
			if envToken == "" {
				return fmt.Errorf("VAULT_TOKEN environment variable not set")
			}
		}
	case "KUBERNETES":
		if vaultConfig.Path == "" || vaultConfig.Role == "" {
			return fmt.Errorf("path and role both required for KUBERNETES auth method")
		}
	case "USERPASS":
		if vaultConfig.Username == "" || vaultConfig.Password == "" || vaultConfig.UserAuthPath == "" {
			return fmt.Errorf("username, password and userAuthPath are required for USERPASS auth method")
		}
	default:
		return fmt.Errorf("unknown Vault secrets auth method: %q", vaultConfig.AuthMethod)
	}

	return nil
}

func (v *VaultDecrypter) setToken() error {
	client, err := v.getVaultClient()
	if err != nil {
		return err
	}
	token, err := v.tokenFetcher.fetchToken(client)
	if err != nil {
		return fmt.Errorf("error fetching vault token - %s", err)
	}
	v.vaultConfig.Token = token
	return nil
}

func (v *VaultDecrypter) getVaultClient() (*api.Logical, error) {
	client, err := v.newAPIClient()
	if err != nil {
		return nil, err
	}
	return client.Logical(), nil
}

func (v *VaultDecrypter) newAPIClient() (*api.Client, error) {
	client, err := api.NewClient(&api.Config{
		Address: v.vaultConfig.Url,
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching vault client: %s", err)
	}
	if v.vaultConfig.Namespace != "" {
		client.SetNamespace(v.vaultConfig.Namespace)
	}
	if v.vaultConfig.Token != "" {
		client.SetToken(v.vaultConfig.Token)
	}
	return client, nil
}

func (v *VaultDecrypter) fetchSecret(client VaultClient) (string, error) {
	path := v.engine + "/" + v.path
	log.Infof("attempting to read secret at KV v1 path: %s", path)
	secretMapping, v1err := client.Read(path)
	if _, ok := v1err.(*json.SyntaxError); ok {
		// some connection errors aren't properly caught, and the vault client tries to parse <nil>
		return "", fmt.Errorf("error fetching secret from vault - check connection to the server: %s",
			v.vaultConfig.Url)
	}

	var v2err error
	if containsRetryableError(v1err, secretMapping) {
		// try again using K/V v2 path
		path = v.engine + "/data/" + v.path
		log.Infof("attempting to read secret at KV v2 path: %s", path)
		secretMapping, v2err = client.Read(path)
	}

	if v2err != nil {
		log.Errorf("error reading secret at KV v1 path and KV v2 path")
		log.Errorf("KV v1 error: %s", v1err)
		log.Errorf("KV v2 error: %s", v2err)
		return "", fmt.Errorf("error fetching secret from vault")
	}

	return v.parseResults(secretMapping)
}

func containsRetryableError(err error, secret *api.Secret) bool {
	if err != nil || secret == nil {
		return true
	}
	warnings := secret.Warnings
	for _, w := range warnings {
		switch {
		case strings.Contains(w, "Invalid path for a versioned K/V secrets engine"):
			return true
		}
	}
	return false
}

func (v *VaultDecrypter) parseResults(secretMapping *api.Secret) (string, error) {
	if secretMapping == nil {
		return "", fmt.Errorf("couldn't find vault path %s under engine %s", v.path, v.engine)
	}

	mapping := secretMapping.Data
	if data, ok := mapping["data"]; ok { // one more nesting of "data" if using K/V v2
		if submap, ok := data.(map[string]interface{}); ok {
			mapping = submap
		}
	}

	decrypted, ok := mapping[v.key].(string)
	if !ok {
		return "", fmt.Errorf("key %q not found at engine: %s, path: %s", v.key, v.engine, v.path)
	}
	log.Debugf("successfully fetched secret")
	return decrypted, nil
}

func DecodeVaultConfig(vaultYaml map[interface{}]interface{}) (*VaultConfig, error) {
	var cfg VaultConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(vaultYaml); err != nil {
		return nil, err
	}

	return &cfg, nil
}
