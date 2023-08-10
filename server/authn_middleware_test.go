package server

import (
	"errors"
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGinEnforceAuthMiddleware(t *testing.T) {
	cases := []struct {
		name                 string
		headers              map[string][]string
		principal            *iam.ArmoryCloudPrincipal
		verifyPrincipalError error
		assertion            func(t *testing.T, ctx *gin.Context, response *http.Response)
	}{
		{
			name: "no bearer token: returns 401",
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.True(t, ctx.IsAborted())
			},
		},
		{
			name: "no principal: returns 401",
			headers: map[string][]string{
				"Authorization": {"Bearer <token>"},
			},
			verifyPrincipalError: errors.New("invalid principal"),
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.True(t, ctx.IsAborted())
			},
		},
		{
			name: "principal is propagated into Gin context: returns 200",
			headers: map[string][]string{
				"Authorization": {"Bearer <token>"},
			},
			principal: &iam.ArmoryCloudPrincipal{
				Name:  "America's #1 Principal",
				OrgId: "org-id",
				EnvId: "env-id",
			},
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)

				principal, err := iam.ExtractPrincipalFromContext(ctx.Request.Context())
				assert.NoError(t, err)
				assert.Equal(t, "America's #1 Principal", principal.Name)
				assert.False(t, ctx.IsAborted())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logger := zap.S()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = &http.Request{
				Header: c.headers,
			}

			authService := mockAuthService{
				principal: c.principal,
				error:     c.verifyPrincipalError,
			}
			ginEnforceAuthMiddleware(authService, logger)(ctx)
			c.assertion(t, ctx, recorder.Result())
		})
	}
}

func TestGinAttemptAuthMiddleware(t *testing.T) {
	cases := []struct {
		name                 string
		headers              map[string][]string
		principal            *iam.ArmoryCloudPrincipal
		verifyPrincipalError error
		assertion            func(t *testing.T, ctx *gin.Context, response *http.Response)
	}{
		{
			name: "no bearer token: returns 200",
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				assert.False(t, ctx.IsAborted())
			},
		},
		{
			name: "no principal: returns 200",
			headers: map[string][]string{
				"Authorization": {"Bearer <token>"},
			},
			verifyPrincipalError: errors.New("invalid principal"),
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				assert.False(t, ctx.IsAborted())
			},
		},
		{
			name: "principal is propagated into Gin context: returns 200",
			headers: map[string][]string{
				"Authorization": {"Bearer <token>"},
			},
			principal: &iam.ArmoryCloudPrincipal{
				Name:  "America's #1 Principal",
				OrgId: "org-id",
				EnvId: "env-id",
			},
			assertion: func(t *testing.T, ctx *gin.Context, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)

				principal, err := iam.ExtractPrincipalFromContext(ctx.Request.Context())
				assert.NoError(t, err)
				assert.Equal(t, "America's #1 Principal", principal.Name)
				assert.False(t, ctx.IsAborted())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = &http.Request{
				Header: c.headers,
			}

			authService := mockAuthService{
				principal: c.principal,
				error:     c.verifyPrincipalError,
			}
			ginAttemptAuthMiddleware(authService)(ctx)
			c.assertion(t, ctx, recorder.Result())
		})
	}
}

type mockAuthService struct {
	principal *iam.ArmoryCloudPrincipal
	error     error
}

func (m mockAuthService) VerifyPrincipalAndSetContext(tokenOrRawHeader string, c *gin.Context) error {
	if m.error != nil {
		return m.error
	}

	if m.principal != nil {
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(c.Request.Context(), m.principal))
	}
	return nil
}
