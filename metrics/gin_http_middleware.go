package metrics

import (
	armoryhttp "github.com/armory-io/lib-go-armory-cloud-commons/http"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func GinHTTPMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		statusCode := c.Writer.Status()
		outcome := "UNKNOWN"
		if statusCode >= 200 && statusCode < 300 {
			outcome = "SUCCESS"
		} else if statusCode >= 400 && statusCode < 500 {
			outcome = "CLIENT_ERROR"
		} else if statusCode >= 500 {
			outcome = "SERVER_ERROR"
		}

		c.Writer.Status()
		uri := c.FullPath()

		tags := map[string]string{
			"uri":           uri,
			"status":        strconv.Itoa(statusCode),
			"client":        armoryhttp.ClientVersionFromContext(c.Request.Context()).Product,
			"clientVersion": armoryhttp.ClientVersionFromContext(c.Request.Context()).Version,
			"outcome":       outcome,
		}

		metrics.TimerWithTags("http.server.requests", tags).Record(time.Now().Sub(start))
	}
}
