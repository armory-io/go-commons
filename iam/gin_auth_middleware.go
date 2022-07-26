package iam

import (
	"context"
	"fmt"
	armoryhttp "github.com/armory-io/lib-go-armory-cloud-commons/http"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func GinAuthMiddleware(ps *ArmoryCloudPrincipalService) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth, err := extractBearerToken(c.Request)
		if err != nil {
			errWriter(c, http.StatusUnauthorized, err.Error())
			return
		}
		// verify principal
		p, err := ps.ExtractAndVerifyPrincipalFromTokenString(strings.TrimPrefix(auth, fmt.Sprintf("%s ", bearerPrefix)))
		if err != nil {
			errWriter(c, http.StatusForbidden, err.Error())
			return
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), principalContextKey{}, *p))
	}
}

func errWriter(c *gin.Context, status int, msg string) {
	c.Header("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	c.JSON(status, armoryhttp.BackstopError{
		Errors: armoryhttp.Errors{{Message: msg}},
	})
	c.Abort()
}
