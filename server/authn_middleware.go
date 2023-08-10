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

// ginEnforceAuthMiddleware extracts an iam.ArmoryCloudPrincipal from the incoming HTTP request.
// If a principal cannot be extracted from the request, the middleware aborts the middleware chain
// and returns a 401.
func ginEnforceAuthMiddleware(as AuthService, log *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := extractPrincipalFromHTTPRequestAndSetContext(c, as); err != nil {
			writeAndLogApiErrorThenAbort(c, err, log)
			c.Abort()
		}
	}
}

// ginAttemptAuthMiddleware attempts to extract an iam.ArmoryCloudPrincipal from the incoming HTTP request,
// but does not abort the middleware chain if it cannot do so.
func ginAttemptAuthMiddleware(as AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = extractPrincipalFromHTTPRequestAndSetContext(c, as)
	}
}

func extractPrincipalFromHTTPRequestAndSetContext(c *gin.Context, as AuthService) serr.Error {
	auth, err := iam.ExtractBearerToken(c.Request)
	if err != nil {
		return serr.NewSimpleErrorWithStatusCode("Failed to extract access token from request", http.StatusUnauthorized, err)
	}

	if err := as.VerifyPrincipalAndSetContext(auth, c); err != nil {
		return serr.NewSimpleErrorWithStatusCode("Failed to verify principal from access token", http.StatusUnauthorized, err)
	}
	return nil
}
