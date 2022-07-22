package iam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	armoryCloudPrincipalClaimNamespace = "https://cloud.armory.io/principal"
	bearerPrefix                       = "Bearer"
	authorizationHeader                = "Authorization"
	proxiedAuthorizationHeader         = "X-Armory-Proxied-Authorization"
)

type principalContextKey struct{}

type ArmoryCloudPrincipalService struct {
	JwtFetcher JwtFetcher
	Validators []PrincipalValidator
}

// PrincipalValidator is used by ArmoryCloudPrincipalMiddleware for principal scope and property validation required for authZ by a service
type PrincipalValidator func(p *ArmoryCloudPrincipal) error

// CreatePrincipalServiceInstance downloads JWKS from the Armory Auth Server & populates the JWK Cache for principal verification
func CreatePrincipalServiceInstance(issuer string, v ...PrincipalValidator) (*ArmoryCloudPrincipalService, error) {
	jt := &JwtToken{
		issuer: issuer,
	}

	// Download JWKs from Armory Auth Server
	if err := jt.Download(); err != nil {
		return nil, err
	}

	return &ArmoryCloudPrincipalService{
		JwtFetcher: jt,
		Validators: v,
	}, nil
}

func (a *ArmoryCloudPrincipalService) ArmoryCloudPrincipalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, err := extractBearerToken(r)
		if err != nil {
			errWriter(w, http.StatusUnauthorized, err.Error())
			return
		}
		// verify principal
		p, err := a.ExtractAndVerifyPrincipalFromTokenString(strings.TrimPrefix(auth, fmt.Sprintf("%s ", bearerPrefix)))
		if err != nil {
			errWriter(w, http.StatusForbidden, err.Error())
			return
		}
		// additional validation (ie scopes, jwt properties)
		for _, validator := range a.Validators {
			if err := validator(p); err != nil {
				errWriter(w, http.StatusForbidden, err.Error())
				return
			}
		}
		// add the principal to the request context for downstream use
		requestWithPrincipal := r.WithContext(context.WithValue(r.Context(), principalContextKey{}, *p))
		next.ServeHTTP(w, requestWithPrincipal)
	})
}

// ExtractPrincipalFromContext can be used by any handler or downstream middleware of the ArmoryCloudPrincipalMiddleware
// to get the encoded principal for manual verification of scopes.
func ExtractPrincipalFromContext(ctx context.Context) (*ArmoryCloudPrincipal, error) {
	v, ok := ctx.Value(principalContextKey{}).(ArmoryCloudPrincipal)
	if !ok {
		return nil, errors.New("unable to extract armory principal from request")
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

func (a *ArmoryCloudPrincipalService) ExtractAndVerifyPrincipalFromTokenString(token string) (*ArmoryCloudPrincipal, error) {
	return a.ExtractAndVerifyPrincipalFromTokenBytes([]byte(token))
}

func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get(authorizationHeader)
	// Prefer the proxied header if it is present from Glados
	if proxiedAuth := r.Header.Get(proxiedAuthorizationHeader); proxiedAuth != "" {
		auth = proxiedAuth
	}

	if auth == "" {
		return "", errors.New("Must provide Authorization header")
	}

	authHeader := strings.Split(auth, fmt.Sprintf("%s ", bearerPrefix))
	if len(authHeader) != 2 {
		return "", errors.New("Malformed token")
	}
	return auth, nil
}

func tokenToPrincipal(untypedPrincipal interface{}, scopes interface{}) (*ArmoryCloudPrincipal, error) {
	principal, ok := untypedPrincipal.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected %s claim format found", armoryCloudPrincipalClaimNamespace)
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
		return nil, fmt.Errorf("unable to decode claim %s: %w", armoryCloudPrincipalClaimNamespace, err)
	}

	// ensure we don't inadvertently deserialize scopes from a fake scopes field in the principal
	if scopes != nil {
		scopeStr, ok := scopes.(string)
		if ok {
			typedPrincipal.Scopes = strings.Split(scopeStr, " ")
		}
	}

	return typedPrincipal, nil
}
