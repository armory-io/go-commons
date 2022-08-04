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

type (
	BackstopError struct {
		ErrorID string `json:"error_id"`
		Errors  Errors `json:"errors"`
	}

	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}

	StatusError struct {
		msg  string
		code int
	}
)

func NewStatusError(msg string, httpCode int) *StatusError {
	return &StatusError{
		msg:  msg,
		code: httpCode,
	}
}

func (se *StatusError) Error() string {
	return se.msg
}

func (se *StatusError) StatusCode() int {
	return se.code
}
