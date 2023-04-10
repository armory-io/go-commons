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

package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	clientcore "github.com/armory-io/go-commons/http/client/core"
	"github.com/armory-io/go-commons/opentelemetry"
	"go.uber.org/fx"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type (
	accessTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int32  `json:"expires_in"`
	}

	AccessToken struct {
		AccessToken string
		TokenType   string
		expiresAt   time.Time
	}

	AccessTokenSupplierConfig struct {
		ClientID       string
		ClientSecret   string
		TokenIssuerURL string
		Audience       string
	}

	AccessTokenSupplierParameters struct {
		fx.In

		Config  AccessTokenSupplierConfig
		Tracing opentelemetry.Configuration `optional:"true"`
	}

	AccessTokenSupplier struct {
		// mu protects access to accessToken.
		mu          *sync.Mutex
		accessToken *AccessToken
		config      AccessTokenSupplierConfig
		http        *http.Client
	}
)

func NewAccessTokenSupplier(params AccessTokenSupplierParameters) *AccessTokenSupplier {
	return &AccessTokenSupplier{
		mu:     &sync.Mutex{},
		config: params.Config,
		http:   clientcore.NewHTTPClient(clientcore.Parameters{Tracing: params.Tracing}),
	}
}

func (s *AccessTokenSupplier) GetRawTokenValue(ctx context.Context) (string, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// GetToken is an alias for GetRawTokenValue.
func (s *AccessTokenSupplier) GetToken(ctx context.Context) (string, error) {
	return s.GetRawTokenValue(ctx)
}

func (s *AccessTokenSupplier) GetAuthorizationHeaderValue(ctx context.Context) (string, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", token.TokenType, token.AccessToken), nil
}

func (s *AccessTokenSupplier) getAccessToken(ctx context.Context) (*AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken == nil || time.Now().After(s.accessToken.expiresAt) {
		token, err := s.fetchNewAccessToken(ctx)
		if err != nil {
			return nil, err
		}
		s.accessToken = token
	}
	return s.accessToken, nil
}

func (s *AccessTokenSupplier) fetchNewAccessToken(ctx context.Context) (*AccessToken, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("audience", s.config.Audience)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.TokenIssuerURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("accept", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected status code while getting token %d", res.StatusCode)
	}
	var accessTokenResponse *accessTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&accessTokenResponse); err != nil {
		return nil, err
	}

	expiresIn := time.Duration(rand.Int31n(accessTokenResponse.ExpiresIn)) * time.Second
	leeway := time.Second * 120
	expiresAt := time.Now().Add(expiresIn - leeway)

	return &AccessToken{
		AccessToken: accessTokenResponse.AccessToken,
		TokenType:   accessTokenResponse.TokenType,
		expiresAt:   expiresAt,
	}, nil
}
