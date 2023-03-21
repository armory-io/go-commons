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

package metrics

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func GinHTTPMiddleware(metrics MetricsSvc) gin.HandlerFunc {
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
			"uri":     uri,
			"status":  strconv.Itoa(statusCode),
			"outcome": outcome,
		}

		metrics.TimerWithTags("http.server.requests", tags).Record(time.Since(start))
	}
}
