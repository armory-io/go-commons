package http

import (
	"context"
	"github.com/armory-io/lib-go-armory-cloud-commons/iam/token"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func NewClient(ctx context.Context, log *zap.SugaredLogger, svc ClientSettings, identity token.Identity) (*Client, error) {
	rt, err := makeClient(ctx, log, svc, identity)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseUrl: strings.TrimSuffix(svc.BaseUrl, "/"),
		c: &http.Client{
			Transport: rt,
		},
	}, nil
}

type Client struct {
	baseUrl string
	c       *http.Client
}

func makeClient(ctx context.Context, log *zap.SugaredLogger, s ClientSettings, identity token.Identity) (http.RoundTripper, error) {
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(s.TimeoutSeconds) * time.Second,
			KeepAlive: time.Duration(s.KeepAliveSeconds) * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          s.MaxIdleConns,
		IdleConnTimeout:       time.Duration(s.IdleConnTimeoutSeconds) * time.Second,
		TLSHandshakeTimeout:   time.Duration(s.TLSHandshakeTimeoutSeconds) * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	cfg, err := GetTLSConfig(s.TLS)
	if err != nil {
		return nil, err
	}
	t.TLSClientConfig = cfg

	return token.GetTokenWrapper(t, identity, log), nil
}

func (s *Client) NewRequest(method, relativeUrl string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, s.baseUrl+relativeUrl, body)
}

func (s *Client) GetClient() *http.Client {
	return s.c
}

func NewDefaultClientSettings() ClientSettings {
	return ClientSettings{
		MaxIdleConns:               10,
		IdleConnTimeoutSeconds:     90,
		TLSHandshakeTimeoutSeconds: 10,
		KeepAliveSeconds:           30,
		TimeoutSeconds:             30,
	}
}

type ClientSettings struct {
	// Client base URL (e.g. https://my-service/some/path/)
	BaseUrl string `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty"`
	// MaxIdleConns controls the maximum number of idle (keep-alive)
	// connections for the given service. Zero means no limit.
	// From http.Transport
	// Defaults to 10
	MaxIdleConns int `yaml:"maxIdleConns,omitempty" json:"maxIdleConns,omitempty"`
	// IdleConnTimeoutSeconds is the maximum amount of time an idle
	// (keep-alive) connection will remain idle before closing
	// itself.
	// Zero means no limit.
	// From http.Transport
	// Defaults to 90s
	IdleConnTimeoutSeconds int32 `yaml:"idleConnTimeoutSeconds,omitempty" json:"idleConnTimeoutSeconds,omitempty"`
	// MaxConnections optionally limits the number of connections to the service.
	// Zero means no limit.
	// Defaults to zero
	MaxConnections int32 `yaml:"maxConnections,omitempty" json:"maxConnections,omitempty"`
	// TLSHandshakeTimeoutSeconds specifies the maximum amount of time waiting to
	// wait for a TLS handshake. Zero means no timeout.
	// From http.Transport
	// Defaults to 10s
	TLSHandshakeTimeoutSeconds int32 `yaml:"tlsHandshakeTimeoutSeconds,omitempty" json:"tlsHandshakeTimeoutSeconds,omitempty"`

	// TimeoutSeconds limits how long we wait for a response
	// Zero means no limit
	// Defaults to 30s
	TimeoutSeconds int32 `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`

	// KeepAliveSeconds specifies the interval between keep-alive
	// probes for an active network connection.
	// If zero, keep-alive probes are sent with a default value
	// (currently 15 seconds), if supported by the protocol and operating
	// system. Network protocols or operating systems that do
	// not support keep-alives ignore this field.
	// If negative, keep-alive probes are disabled.
	// ^ from net.dial
	// Defaults to 30s
	KeepAliveSeconds int32 `yaml:"keepAliveSeconds,omitempty" json:"keepAliveSeconds,omitempty"`

	TLS *ClientTLSSettings `yaml:"tls,omitempty" json:"tls,omitempty"`
}
