/*
 * Copyright (c) 2022 Armory, Inc
 *   National Electronics and Computer Technology Center, Thailand
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package token

import (
	"context"
	"go.uber.org/zap"
	"net/http"
)

type credentials interface {
	GetRequestMetadata(ctx context.Context, uri string) (map[string]string, error)
}

func GetTokenWrapper(base http.RoundTripper, auth Identity, log *zap.SugaredLogger) http.RoundTripper {
	creds := GetCredentials(auth, log)
	if creds == nil {
		return base
	}
	return &wrappedTokenSource{
		base:  base,
		creds: creds,
	}
}

type wrappedTokenSource struct {
	base  http.RoundTripper
	creds credentials
}

func (w *wrappedTokenSource) RoundTrip(r *http.Request) (*http.Response, error) {
	m, err := w.creds.GetRequestMetadata(context.TODO(), r.RequestURI)
	if err != nil {
		return nil, err
	}
	for k, v := range m {
		r.Header.Set(k, v)
	}
	return w.base.RoundTrip(r)
}
