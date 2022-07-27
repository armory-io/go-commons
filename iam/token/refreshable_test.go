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
	"crypto/rand"
	"crypto/rsa"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type fakeTokenGetter struct {
	calls  int
	expSec int32
}

func (f *fakeTokenGetter) GetToken(ctx context.Context) (string, *time.Time, error) {
	ts, tk := makeTestToken(time.Now().Add(time.Duration(f.expSec) * time.Second))
	exp := tk.Expiration()
	f.calls = f.calls + 1
	return ts, &exp, nil
}

func TestRefreshable(t *testing.T) {
	f := &fakeTokenGetter{
		// Make token that expires 2 seconds after their issued
		expSec: 2,
	}

	// Refreshable token with a 1 second leeway
	r := newRefreshableTokenCredentials(f, 1)
	ctx := context.TODO()

	// Get token
	m, err := r.GetRequestMetadata(ctx, "")
	assert.Nil(t, err)
	assert.NotNil(t, m)
	h := m["authorization"]
	assert.NotEmpty(t, h)
	assert.Equal(t, f.calls, 1)

	// Get token again
	_, err = r.GetRequestMetadata(ctx, "")
	assert.Nil(t, err)
	// should not make a new call because token has not expired
	assert.Equal(t, f.calls, 1)

	// wait until leeway makes us get the token again
	time.Sleep(1 * time.Second)

	// get token one more time
	_, _ = r.GetRequestMetadata(ctx, "")
	assert.Nil(t, err)
	// should have given a new call
	assert.Equal(t, f.calls, 2)
}

func makeTestToken(exp time.Time) (string, jwt.Token) {
	tk := jwt.New()
	tk.Set(jwt.AudienceKey, "my-audience")
	tk.Set(jwt.IssuedAtKey, time.Now())
	tk.Set(jwt.ExpirationKey, exp)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", nil
	}

	signed, err := jwt.Sign(tk, jwa.RS256, key)
	if err != nil {
		return "", nil
	}
	return string(signed), tk
}
