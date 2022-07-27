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
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type testTokenServer struct {
	req            *http.Request
	form           url.Values
	cannedResponse []byte
}

func (tt *testTokenServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tt.req = r
	r.ParseForm()
	tt.form = r.Form
	w.Write(tt.cannedResponse)
}

func TestGetTokenServer(t *testing.T) {
	const clientId = "my-client-id"
	const secret = "my-secret"
	const audience = "my-audience"

	tk, _ := makeTestToken(time.Now().Add(10 * time.Second))
	tkExpired, _ := makeTestToken(time.Now().Add(-1 * time.Second))
	cases := []struct {
		cloud          ArmoryCloud
		cannedResponse []byte
		check          func(t2 *testing.T, tk string, exp *time.Time, getErr error, req *http.Request, form url.Values)
	}{
		{
			cloud: ArmoryCloud{
				ClientId: clientId,
				Secret:   secret,
				Audience: audience,
				Verify:   true,
			},
			cannedResponse: []byte(fmt.Sprintf("{\"access_token\": \"%s\"}", tk)),
			check: func(t2 *testing.T, tk string, exp *time.Time, getErr error, req *http.Request, form url.Values) {
				assert.Nil(t2, getErr)
				assert.NotNil(t2, exp)
				assert.NotEmpty(t2, tk)
				assert.NotNil(t2, req)
				assert.Equal(t2, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				assert.Equal(t2, "application/json", req.Header.Get("accept"))
				assert.Equal(t2, "client_credentials", form.Get("grant_type"))
				assert.Equal(t2, clientId, form.Get("client_id"))
				assert.Equal(t2, secret, form.Get("client_secret"))
				assert.Equal(t2, audience, form.Get("audience"))
			},
		},
		{
			cloud: ArmoryCloud{
				ClientId: clientId,
				Secret:   secret,
				Audience: audience,
				Verify:   false,
			},
			cannedResponse: []byte(fmt.Sprintf("{\"access_token\": \"%s\"}", tkExpired)),
			check: func(t2 *testing.T, tk string, exp *time.Time, getErr error, req *http.Request, form url.Values) {
				assert.Nil(t2, getErr)
				assert.NotNil(t2, exp)
				assert.NotEmpty(t2, tk)
				assert.NotNil(t2, req)
				assert.Equal(t2, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				assert.Equal(t2, "application/json", req.Header.Get("accept"))
				assert.Equal(t2, "client_credentials", form.Get("grant_type"))
				assert.Equal(t2, clientId, form.Get("client_id"))
				assert.Equal(t2, secret, form.Get("client_secret"))
				assert.Equal(t2, audience, form.Get("audience"))
			},
		},
		{
			cloud: ArmoryCloud{
				ClientId: clientId,
				Secret:   secret,
				Audience: audience,
				Verify:   true,
			},
			cannedResponse: []byte(fmt.Sprintf("{\"access_token\": \"%s\"}", tkExpired)),
			check: func(t2 *testing.T, tk string, exp *time.Time, getErr error, req *http.Request, form url.Values) {
				assert.NotNil(t2, getErr)
				assert.Equal(t2, "exp not satisfied", getErr.Error())
			},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("test-%d", i), func(t2 *testing.T) {
			tt := testTokenServer{
				cannedResponse: c.cannedResponse,
			}
			s := httptest.NewServer(&tt)
			defer s.Close()

			auth := Identity{
				Armory: c.cloud,
			}

			auth.Armory.TokenIssuerUrl = s.URL
			log := zap.New(nil).Sugar()

			g := getTokenGetter(auth, log)
			assert.NotNil(t2, g)
			tk, exp, err := g.GetToken(context.TODO())
			c.check(t2, tk, exp, err, tt.req, tt.form)
		})
	}
}
