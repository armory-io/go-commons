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

import "github.com/armory-io/go-commons/http"

type SPAConfiguration struct {
	Enabled   bool
	Prefix    string
	Directory string
}

type Configuration struct {
	RequestLogging RequestLoggingConfiguration
	SPA            SPAConfiguration
	HTTP           http.HTTP
	Management     http.HTTP
}

// RequestLoggingConfiguration enable request logging, by default all requests are logged.
// See fields for options on filtering what is logged
type RequestLoggingConfiguration struct {
	// Enabled if set to true a request logging middleware will be applied to all requests
	Enabled bool
	// BlockList configure a set of endpoints to skip request logging on, such as the health check endpoints
	BlockList []string
	// Disable2XX if enabled requests inside the 200-299 range will not be logged
	Disable2XX bool
	// Disable3XX if enabled requests inside the 300-399 range will not be logged
	Disable3XX bool
	// Disable4XX if enabled requests inside the 400-499 range will not be logged
	Disable4XX bool
	// Disable5XX if enabled requests inside the 500-599 range will not be logged
	Disable5XX bool
}
