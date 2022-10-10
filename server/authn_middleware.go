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
