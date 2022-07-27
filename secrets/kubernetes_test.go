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

package secrets

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewKubernetesSecretDecrypter(t *testing.T) {
	cases := []struct {
		in  string
		err string
	}{
		{
			in:  "blah",
			err: K8sGenericMalformedKeyError,
		},
		{
			in:  "!s:foo",
			err: K8sGenericMalformedKeyError,
		},
		{
			in:  "k:secret-key",
			err: K8sSecretNameMissingError,
		},
		{
			in:  "n:secret-key",
			err: K8sSecretKeyMissingError,
		},
		{
			in:  "n:kubernetes-secret-name!k:secret-key",
			err: "failed to determine namespace, you must supply the `!ns:` key or be running on a pod where /var/run/secrets/kubernetes.io/serviceaccount/namespace is defined",
		},
		{
			in:  "ns:foo!n:kubernetes-secret-name!k:secret-key",
			err: "",
		},
		{
			in:  "ns:foo!n:kubernetes-secret-name!k:secret-key!dne:bar",
			err: K8sGenericMalformedKeyError,
		},
	}

	for _, c := range cases {
		_, err := NewKubernetesSecretDecrypter(context.TODO(), false, c.in)
		eMsg := ""
		if err != nil {
			eMsg = err.Error()
		}
		assert.Equal(t, c.err, eMsg)
	}
}
