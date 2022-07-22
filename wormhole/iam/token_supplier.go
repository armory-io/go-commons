package iam

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int32  `json:"expires_in"`
}

type AccessToken struct {
	AccessToken string
	TokenType   string
	expiresAt   time.Time
}

type AccessTokenSupplierConfig struct {
	ClientId       string
	ClientSecret   string
	TokenIssuerUrl string
	Audience       string
}

type AccessTokenSupplier struct {
	accessToken *AccessToken
	Config      *AccessTokenSupplierConfig
}

func (s *AccessTokenSupplier) GetRawTokenValue() (string, error) {
	token, err := s.getAccessToken()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (s *AccessTokenSupplier) GetAuthorizationHeaderValue() (string, error) {
	token, err := s.getAccessToken()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", token.TokenType, token.AccessToken), nil
}

func (s *AccessTokenSupplier) getAccessToken() (*AccessToken, error) {
	if s.accessToken == nil || time.Now().After(s.accessToken.expiresAt) {
		aT, err := s.fetchNewAccessToken()
		if err != nil {
			return nil, err
		}
		s.accessToken = aT
	}
	return s.accessToken, nil
}

func (s *AccessTokenSupplier) fetchNewAccessToken() (*AccessToken, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", s.Config.ClientId)
	data.Set("client_secret", s.Config.ClientSecret)
	data.Set("audience", s.Config.Audience)
	req, err := http.NewRequest(http.MethodPost, s.Config.TokenIssuerUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("accept", "application/json")
	c := &http.Client{}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected status code while getting token %d", res.StatusCode)
	}
	var accessTokenResponse *accessTokenResponse
	err = json.NewDecoder(res.Body).Decode(&accessTokenResponse)
	if err != nil {
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
