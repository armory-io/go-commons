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

package token

import (
	"context"
	"time"
)

func newRefreshableTokenCredentials(getter tokenGetter, expLeewaySec int64) *refreshableTokenCredentials {
	return &refreshableTokenCredentials{
		getter:       getter,
		expLeewaySec: expLeewaySec,
	}
}

type refreshableTokenCredentials struct {
	token        string
	exp          *time.Time
	getter       tokenGetter
	expLeewaySec int64
}

func (r *refreshableTokenCredentials) RequireTransportSecurity() bool {
	return true
}

func (r *refreshableTokenCredentials) GetRequestMetadata(ctx context.Context, uri string) (map[string]string, error) {
	// if never expired or if the expiration is before now + leeway
	if r.exp == nil || r.exp.Before(time.Now().Add(time.Duration(r.expLeewaySec)*time.Second)) {
		var err error
		r.token, r.exp, err = r.getter.GetToken(ctx)
		if err != nil {
			return nil, err
		}
	}
	return map[string]string{
		"authorization": "Bearer " + r.token,
	}, nil
}
