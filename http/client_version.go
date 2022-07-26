package http

import (
	"context"
	"github.com/gin-gonic/gin"
	"strings"
)

const (
	armoryClientHeader = "X-Armory-Client"
)

type (
	clientVersionContextKey struct{}

	ClientVersion struct {
		Product string
		Version string
	}
)

func GinClientVersionMiddleware(c *gin.Context) {
	parts := strings.Split(c.GetHeader(armoryClientHeader), "/")
	if len(parts) != 2 {
		return
	}

	cv := ClientVersion{
		Product: parts[0],
		Version: parts[1],
	}

	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), clientVersionContextKey{}, cv))
}

func ClientVersionFromContext(ctx context.Context) ClientVersion {
	cv, ok := ctx.Value(clientVersionContextKey{}).(ClientVersion)
	if !ok {
		return ClientVersion{
			Product: "unset",
			Version: "unset",
		}
	}
	return cv
}
