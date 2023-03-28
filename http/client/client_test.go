package client

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerTokenRoundTripper(t *testing.T) {
	rt := bearerTokenRoundTripper{
		base:          http.DefaultTransport,
		tokenSupplier: mockTokenSupplier{},
	}

	client := &http.Client{
		Transport: &rt,
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer my-token", request.Header.Get("Authorization"))
		writer.WriteHeader(http.StatusNoContent)
	}))

	_, err := client.Get(server.URL)
	assert.NoError(t, err)
}

type mockTokenSupplier struct{}

func (m mockTokenSupplier) GetToken() (string, error) {
	return "my-token", nil
}
