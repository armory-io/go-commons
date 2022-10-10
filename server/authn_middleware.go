package server

import (
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

func ginAuthMiddleware(ps *iam.ArmoryCloudPrincipalService, log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// extract access token from request
		auth, err := iam.ExtractBearerToken(c.Request)
		if err != nil {
			apiErr := serr.NewSimpleErrorWithStatusCode(
				"Failed to extract access token from request", http.StatusUnauthorized, err)
			writeAndLogApiErrorThenAbort(apiErr, c, log)
			c.Abort()
			return
		}
		// verify principal from access token
		if err := ps.VerifyPrincipalAndSetContext(auth, c); err != nil {
			apiErr := serr.NewSimpleErrorWithStatusCode(
				"Failed to verify principal from access token", http.StatusUnauthorized, err)
			writeAndLogApiErrorThenAbort(apiErr, c, log)
			c.Abort()
			return
		}
	}
}
