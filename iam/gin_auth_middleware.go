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

package iam

import (
	"context"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func GinAuthMiddleware(ps *ArmoryCloudPrincipalService, allowWithoutAuthList []string) gin.HandlerFunc {

	allowList := make(map[string]bool)
	for _, route := range allowWithoutAuthList {
		allowList[route] = true
	}

	return func(c *gin.Context) {
		if allowList[c.FullPath()] {
			return
		}
		auth, err := extractBearerToken(c.Request)
		if err != nil {
			ginErrWriter(c, http.StatusUnauthorized, err.Error())
			return
		}
		// verify principal
		p, err := ps.ExtractAndVerifyPrincipalFromTokenString(strings.TrimPrefix(auth, fmt.Sprintf("%s ", bearerPrefix)))
		if err != nil {
			ginErrWriter(c, http.StatusForbidden, err.Error())
			return
		}

		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), principalContextKey{}, *p))
	}
}

func ginErrWriter(c *gin.Context, status int, msg string) {
	c.Header("Content-Type", "application/json")
	c.Writer.WriteHeader(status)
	c.JSON(status, armoryhttp.BackstopError{
		Errors: armoryhttp.Errors{{Message: msg}},
	})
	c.Abort()
}
