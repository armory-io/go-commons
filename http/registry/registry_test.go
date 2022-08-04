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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var (
	defaultPrincipal = &iam.ArmoryCloudPrincipal{
		Type:  "user",
		OrgId: "org",
		EnvId: "env",
	}
)

type (
	request struct {
		Message string `json:"message"`
	}

	response struct {
		Message string
	}
)

func TestHandlerRegistry(t *testing.T) {
	cases := []struct {
		name                string
		handler             Handler
		principal           *iam.ArmoryCloudPrincipal
		request             func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error)
		assertion           func(t *testing.T, responseBytes []byte)
		expectedStatusCode  int
		expectedContentType string
	}{
		{
			name: "request / response handler with GET",
			handler: NewRequestResponseHandler(
				func(ctx context.Context, r request) (*response, error) {
					if r.Message != "we love fish" {
						return nil, fmt.Errorf("unexpected message: %s", r.Message)
					}

					return &response{
						Message: "especially raw fish",
					}, nil
				},
				HandlerConfig{
					Path:       "/sushi",
					Method:     http.MethodGet,
					Validators: []Validator{allowAll},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, fmt.Sprintf("%s%s", server.URL, config.Path+"?message="+url.QueryEscape("we love fish")), nil)
			},
			assertion: func(t *testing.T, responseBytes []byte) {
				var res response
				assert.NoError(t, json.Unmarshal(responseBytes, &res))
				assert.Equal(t, "especially raw fish", res.Message)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "request / response handler with GET path param",
			handler: NewRequestResponseHandler(
				func(ctx context.Context, msg string) (*response, error) {
					if msg != "the-smaller-the-plate" {
						return nil, fmt.Errorf("unexpected message: %s", msg)
					}

					return &response{
						Message: "the better",
					}, nil
				},
				HandlerConfig{
					Path:       "/tapas/:description",
					Method:     http.MethodGet,
					Validators: []Validator{allowAll},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, server.URL+"/tapas/the-smaller-the-plate", nil)
			},
			assertion: func(t *testing.T, responseBytes []byte) {
				var res response
				assert.NoError(t, json.Unmarshal(responseBytes, &res))
				assert.Equal(t, "the better", res.Message)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "request / response handler with POST",
			handler: NewRequestResponseHandler(
				func(ctx context.Context, r request) (*response, error) {
					if r.Message != "we love pasta" {
						return nil, fmt.Errorf("unexpected message: %s", r.Message)
					}
					return &response{
						Message: "and spaghetti and meatballs",
					}, nil
				},
				HandlerConfig{
					Path:       "/pasta",
					Method:     http.MethodPost,
					StatusCode: http.StatusAccepted,
					Validators: []Validator{allowAll},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				reqBody := &request{
					Message: "we love pasta",
				}

				b, err := json.Marshal(reqBody)
				if err != nil {
					return nil, err
				}

				return http.NewRequestWithContext(ctx, config.Method, server.URL+config.Path, bytes.NewReader(b))
			},
			assertion: func(t *testing.T, responseBytes []byte) {
				var res response
				assert.NoError(t, json.Unmarshal(responseBytes, &res))
				assert.Equal(t, "and spaghetti and meatballs", res.Message)
			},
			expectedStatusCode: http.StatusAccepted,
		},
		{
			name: "request handler with 202 response",
			handler: NewRequestHandler(
				func(ctx context.Context, r request) error {
					if r.Message != "cabbage" {
						return fmt.Errorf("unexpected message: %s", r.Message)
					}
					return nil
				},
				HandlerConfig{
					Path:       "/kimchi",
					Method:     http.MethodPost,
					StatusCode: http.StatusAccepted,
					Validators: []Validator{allowAll},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, fmt.Sprintf("%s%s", server.URL, config.Path+"?message=cabbage"), nil)
			},
			expectedStatusCode: http.StatusAccepted,
		},
		{
			name: "request handler with error",
			handler: NewRequestHandler(
				func(ctx context.Context, r request) error {
					return armoryhttp.NewStatusError("nous n'aimons pas les steak-frites", http.StatusUnprocessableEntity)
				},
				HandlerConfig{
					Path:       "/steak-frites",
					Method:     http.MethodPost,
					StatusCode: http.StatusAccepted,
					Validators: []Validator{allowAll},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, fmt.Sprintf("%s%s", server.URL, config.Path), nil)
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "request handler with authorization error",
			handler: NewRequestHandler(
				func(ctx context.Context, r request) error {
					return armoryhttp.NewStatusError("ketchup is for eggs!", http.StatusUnprocessableEntity)
				},
				HandlerConfig{
					Path:       "/ketchup",
					Method:     http.MethodPost,
					StatusCode: http.StatusAccepted,
					Validators: []Validator{func(p *iam.ArmoryCloudPrincipal) (string, bool) {
						for _, scope := range p.Scopes {
							if scope == "apply:ketchup" {
								return "", true
							}
						}
						return "principal must have 'apply:ketchup' scope to access /ketchup", false
					}},
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, fmt.Sprintf("%s%s", server.URL, config.Path), nil)
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name: "request handler with scope validation",
			handler: NewRequestHandler(
				func(ctx context.Context, r request) error {
					return nil
				},
				HandlerConfig{
					Path:       "/ketchup",
					Method:     http.MethodPost,
					StatusCode: http.StatusAccepted,
					Validators: []Validator{func(p *iam.ArmoryCloudPrincipal) (string, bool) {
						for _, scope := range p.Scopes {
							if scope == "apply:ketchup" {
								return "", true
							}
						}
						return "principal must have 'apply:ketchup' scope to access /ketchup", false
					}},
				},
			),
			principal: &iam.ArmoryCloudPrincipal{
				Type:   "user",
				OrgId:  "org",
				EnvId:  "env",
				Scopes: []string{"apply:ketchup"},
			},
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, fmt.Sprintf("%s%s", server.URL, config.Path), nil)
			},
			expectedStatusCode: http.StatusAccepted,
		},
		{
			name: "unknown path",
			handler: NewRequestHandler(
				func(ctx context.Context, r request) error {
					return fmt.Errorf("what are YOU doing here")
				},
				HandlerConfig{
					Method: http.MethodGet,
					Path:   "/mayonnaise",
				},
			),
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, server.URL+"/mustard", nil)
			},
			expectedStatusCode:  http.StatusNotFound,
			expectedContentType: "text/plain", // just gets a 404 Not Found page
		},
		{
			name: "default unauthorized if no validators present",
			handler: NewRequestHandler(
				func(ctx context.Context, _ string) error {
					return fmt.Errorf("you shouldn't have eaten it, Homer")
				},
				HandlerConfig{
					Method: http.MethodGet,
					Path:   "/fugu-fish",
				},
			),
			principal: defaultPrincipal,
			request: func(ctx context.Context, config HandlerConfig, server *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(ctx, config.Method, server.URL+config.Path, nil)
			},
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()

			g := gin.Default()
			g.Use(func(gc *gin.Context) {
				if c.principal != nil {
					gc.Request = gc.Request.WithContext(iam.WithPrincipal(gc.Request.Context(), *c.principal))
				}
			})

			r := &HandlerRegistry{
				log: zap.New(nil).Sugar(),
				gin: g,
			}

			r.RegisterHandlers(c.handler)

			s := httptest.NewServer(g)

			client := http.DefaultClient

			req, err := c.request(ctx, c.handler.Config(), s)
			assert.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)

			if c.expectedContentType == "" {
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			} else {
				assert.Equal(t, c.expectedContentType, resp.Header.Get("Content-Type"))
			}

			responseBytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			if c.assertion != nil {
				c.assertion(t, responseBytes)
			}
		})
	}
}

func allowAll(principal *iam.ArmoryCloudPrincipal) (string, bool) {
	return "", true
}
