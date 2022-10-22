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

package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	ClientAuthNone    ClientAuthType = "none"
	ClientAuthWant    ClientAuthType = "want"
	ClientAuthNeed    ClientAuthType = "need"
	ClientAuthAny     ClientAuthType = "any"
	ClientAuthRequest ClientAuthType = "request"
)

type (
	Configuration struct {
		HTTP HTTP
	}

	HTTP struct {
		Prefix string
		Host   string
		Port   uint32
		SSL    SSL
	}

	SSL struct {
		// Enable SSL
		Enabled bool
		// Certificate file, can be just a PEM of cert + key or just the cert in which case you'll also need
		// to provide the key file
		CertFile string
		// Key file if the cert file doesn't provide it
		KeyFile string
		// Key password if the key is encrypted
		KeyPassword string
		// when using mTLS, CA PEM. If not provided, it will default to the certificate of the server as a CA
		CAcertFile string
		// Client auth requested (none, want, need, any, request)
		ClientAuth ClientAuthType
	}

	ClientAuthType string

	Server struct {
		config Configuration
		server *http.Server
	}
)

func (s Configuration) GetAddr() string {
	return fmt.Sprintf("%s:%d", s.HTTP.Host, s.HTTP.Port)
}

func NewServer(config Configuration) *Server {
	return &Server{
		config: config,
	}
}

// Start starts the server on the configured port
func (s *Server) Start(router http.Handler) error {
	if !s.config.HTTP.SSL.Enabled {
		return s.startHttp(router)
	}
	return s.startTls(router)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) startHttp(router http.Handler) error {
	s.server = &http.Server{
		Addr:    s.config.GetAddr(),
		Handler: router,
	}
	return s.server.ListenAndServe()
}

func (s *Server) startTls(router http.Handler) error {
	tlsConfig, err := s.tlsConfig()
	if err != nil {
		return err
	}

	certMode := s.getClientCertMode()
	if certMode != tls.NoClientCert {
		// With mTLS, we'll parse our PEM to discover CAs with which to validate client certificates
		caFile := s.config.HTTP.SSL.CAcertFile
		if caFile == "" {
			// Fall back to cert file - could be a combined PEM (e.g. self signed)
			caFile = s.config.HTTP.SSL.CertFile
		} else if err := CheckFileExists(caFile); err != nil {
			return fmt.Errorf("error with certificate authority file %s: %w", caFile, err)
		}

		// Create a CA certificate pool and add our server certificate
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = certMode
	}

	// Discover the server name based on given certificates
	tlsConfig.BuildNameToCertificate()

	// Create a Server instance to listen on port 8443 with the TLS config
	s.server = &http.Server{
		Addr:      s.config.GetAddr(),
		Handler:   router,
		TLSConfig: tlsConfig,
	}

	// Listen to HTTPS connections with the server certificate and wait
	return s.server.ListenAndServeTLS("", "")
}

func (s *Server) getClientCertMode() tls.ClientAuthType {
	switch s.config.HTTP.SSL.ClientAuth {
	case ClientAuthWant:
		return tls.VerifyClientCertIfGiven
	case ClientAuthNeed:
		return tls.RequireAndVerifyClientCert
	case ClientAuthAny:
		return tls.RequireAnyClientCert
	case ClientAuthRequest:
		return tls.RequestClientCert
	default:
		return tls.NoClientCert
	}
}

// tlsConfig prepares the TLS config of the server
// certFile must contain the certificate of the server. It can also contain the private key (optionally encrypted)
// keyFile is needed if the certFile doesn't contain the private key. It can also be encrypted.
func (s *Server) tlsConfig() (*tls.Config, error) {
	c, err := GetX509KeyPair(s.config.HTTP.SSL.CertFile, s.config.HTTP.SSL.KeyFile, s.config.HTTP.SSL.KeyPassword)
	if err != nil {
		return nil, fmt.Errorf("error with certificate file %s: %w", s.config.HTTP.SSL.CertFile, err)
	}
	return &tls.Config{
		Certificates:             []tls.Certificate{c},
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}, nil
}
