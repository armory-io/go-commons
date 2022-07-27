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
