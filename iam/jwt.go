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
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

const (
	scopeClaim      = "scope"
	subject         = "sub"
	issuer          = "iss"
	authorizedParty = "azp"
)

type JwtFetcher interface {
	Download() error
	Fetch(token []byte) (interface{}, interface{}, error)
}

type JwtToken struct {
	jwkFetcher *jwk.AutoRefresh
	issuer     string
}

func (j *JwtToken) Download() error {
	// Download JWKs from Armory Auth Server
	ctx := context.Background()
	ar := jwk.NewAutoRefresh(ctx)

	// Tell *jwk.AutoRefresh that we only want to refresh this JWKS
	// when it needs to (based on Cache-Control or Expires header from
	// the HTTP response). If the calculated minimum refresh interval is less
	// than 15 minutes, don't go refreshing any earlier than 15 minutes.
	ar.Configure(j.issuer, jwk.WithMinRefreshInterval(15*time.Minute))

	// Refresh the JWKS once before getting into the main loop.
	// This allows you to check if the JWKS is available before we start
	// a long-running program
	if _, err := ar.Refresh(ctx, j.issuer); err != nil {
		return err
	}
	j.jwkFetcher = ar
	return nil
}

func (j *JwtToken) Fetch(token []byte) (interface{}, interface{}, error) {
	jwkSet, err := j.jwkFetcher.Fetch(context.Background(), j.issuer)
	if err != nil {
		return nil, nil, err
	}

	parsedJwt, err := jwt.Parse(token,
		jwt.WithKeySet(jwkSet),
		jwt.WithValidate(true),
	)
	if err != nil {
		return nil, nil, err
	}

	untypedPrincipal, wasClaimPresent := parsedJwt.Get(ArmoryCloudPrincipalClaimNamespace)
	if !wasClaimPresent {
		return nil, nil, errors.New("required armory cloud principal claim was missing from token")
	}
	untypedPrincipal.(map[string]interface{})[subject] = parsedJwt.Subject()
	untypedPrincipal.(map[string]interface{})[issuer] = parsedJwt.Issuer()
	azp, provided := parsedJwt.Get(authorizedParty)
	if provided {
		untypedPrincipal.(map[string]interface{})[authorizedParty] = azp
	}

	scopes, _ := parsedJwt.Get(scopeClaim)

	return untypedPrincipal, scopes, nil
}
