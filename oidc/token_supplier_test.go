package oidc

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessTokenSupplier(t *testing.T) {
	oidcServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.NoError(t, json.NewEncoder(writer).Encode(accessTokenResponse{
			AccessToken: "my-token",
			TokenType:   "bearer",
			ExpiresIn:   1000,
		}))
	}))

	supplier := NewAccessTokenSupplier(AccessTokenSupplierParameters{
		Config: AccessTokenSupplierConfig{
			ClientID:       "id",
			ClientSecret:   "secret",
			TokenIssuerURL: oidcServer.URL,
			Audience:       "audience",
		},
	})

	token, err := supplier.GetToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "my-token", token)
}
