package server

import (
	"github.com/armory-io/go-commons/iam"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

func ginAuthMiddleware(ps *iam.ArmoryCloudPrincipalService, log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// extract access token from request
		auth, err := iam.ExtractBearerToken(c.Request)
		if err != nil {
			apiErr := NewErrorResponseFromApiError(APIError{
				Message:        "Failed to extract access token from request",
				HttpStatusCode: http.StatusUnauthorized,
			}, WithCause(err))
			writeAndLogApiErrorThenAbort(apiErr, c, log)
			c.Abort()
			return
		}
		// verify principal from access token
		if err := ps.VerifyPrincipalAndSetContext(auth, c); err != nil {
			apiErr := NewErrorResponseFromApiError(APIError{
				Message:        "Failed to verify principal from access token",
				HttpStatusCode: http.StatusUnauthorized,
			}, WithCause(err))
			writeAndLogApiErrorThenAbort(apiErr, c, log)
			c.Abort()
			return
		}
	}
}
