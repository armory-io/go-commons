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

package iam

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"strings"
)

const (
	ArmoryCloudPrincipalClaimNamespace = "https://cloud.armory.io/principal"
	bearerPrefix                       = "Bearer"
	authorizationHeader                = "Authorization"
	proxiedAuthorizationHeader         = "X-Armory-Proxied-Authorization"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNoPrincipal  = errors.New("unable to extract armory principal from request")
)

type principalContextKey struct{}

type ArmoryCloudPrincipalService struct {
	JwtFetcher JwtFetcher
}

// New creates an ArmoryCloudPrincipalService. It downloads JWKS from the Armory Auth Server & populates the JWK Cache for principal verification.
func New(settings Configuration) (*ArmoryCloudPrincipalService, error) {
	jt := &JwtToken{
		issuer: settings.JWT.JWTKeysURL,
	}

	// Download JWKs from Armory Auth Server
	if err := jt.Download(); err != nil {
		return nil, err
	}

	return &ArmoryCloudPrincipalService{
		JwtFetcher: jt,
	}, nil
}

type valuer interface {
	Value(any) any
}

// ExtractPrincipalFromContext can be used by any handler or downstream middleware of the ArmoryCloudPrincipalMiddleware
// to get the encoded principal for manual verification of scopes.
func ExtractPrincipalFromContext(ctx valuer) (*ArmoryCloudPrincipal, error) {
	v, ok := ctx.Value(principalContextKey{}).(ArmoryCloudPrincipal)
	if !ok {
		return nil, ErrNoPrincipal
	}
	return &v, nil
}

func (a *ArmoryCloudPrincipalService) ExtractAndVerifyPrincipalFromTokenBytes(token []byte) (*ArmoryCloudPrincipal, error) {
	parsedJwt, scopes, err := a.JwtFetcher.Fetch(token)
	if err != nil {
		return nil, err
	}

	return tokenToPrincipal(parsedJwt, scopes)
}

func (a *ArmoryCloudPrincipalService) VerifyPrincipalAndSetContext(tokenOrRawHeader string, c *gin.Context) error {
	token := strings.TrimSpace(tokenOrRawHeader)
	if strings.Contains(tokenOrRawHeader, bearerPrefix) {
		token = strings.TrimPrefix(token, fmt.Sprintf("%s ", bearerPrefix))
	}
	p, err := a.ExtractAndVerifyPrincipalFromTokenString(token)
	if err != nil {
		return err
	}
	c.Request = c.Request.WithContext(DangerouslyWriteUnverifiedPrincipalToContext(c.Request.Context(), p))
	return nil
}

// DangerouslyWriteUnverifiedPrincipalToContext is exposed for easily injecting stub principals into context for testing
func DangerouslyWriteUnverifiedPrincipalToContext(c context.Context, principal *ArmoryCloudPrincipal) context.Context {
	return context.WithValue(c, principalContextKey{}, *principal)
}

func (a *ArmoryCloudPrincipalService) ExtractAndVerifyPrincipalFromTokenString(token string) (*ArmoryCloudPrincipal, error) {
	return a.ExtractAndVerifyPrincipalFromTokenBytes([]byte(token))
}

func ExtractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get(authorizationHeader)
	// Prefer the proxied header if it is present from Glados
	if proxiedAuth := r.Header.Get(proxiedAuthorizationHeader); proxiedAuth != "" {
		auth = proxiedAuth
	}

	if auth == "" {
		return "", errors.New("must provide Authorization header")
	}

	authHeader := strings.Split(auth, fmt.Sprintf("%s ", bearerPrefix))
	if len(authHeader) != 2 {
		return "", errors.New("malformed token")
	}
	return auth, nil
}

func tokenToPrincipal(untypedPrincipal any, scopes any) (*ArmoryCloudPrincipal, error) {
	principal, ok := untypedPrincipal.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected %s claim format found", ArmoryCloudPrincipalClaimNamespace)
	}

	var typedPrincipal *ArmoryCloudPrincipal

	cfg := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &typedPrincipal,
		TagName:  "json",
	}
	decoder, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to configure token decoder: %w", err)
	}
	if err := decoder.Decode(principal); err != nil {
		return nil, fmt.Errorf("unable to decode claim %s: %w", ArmoryCloudPrincipalClaimNamespace, err)
	}

	// ensure we don't inadvertently deserialize scopes from a fake scopes field in the principal
	if scopes != nil {
		scopeStr, ok := scopes.(string)
		if ok {
			typedPrincipal.Scopes = append(typedPrincipal.Scopes, strings.Split(scopeStr, " ")...)
		}
	}

	return typedPrincipal, nil
}
