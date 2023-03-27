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

package proxy

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRetry(t *testing.T) {
	counter := 3
	wormhole := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if counter > 0 {
			counter--
			writer.WriteHeader(500)
			return
		}

		if err := json.NewEncoder(writer).Encode(&KubernetesCredentials{
			Host: "success",
		}); err != nil {
			writer.WriteHeader(500)
		}
	}))

	client := New(WormholeServiceParameters{
		Client:    &http.Client{},
		BaseURL:   wormhole.URL,
		Overrides: &SessionOverrides{},
		Logger:    zap.S(),
	})

	creds, err := client.GetKubernetesClusterCredentialsFromAgent(context.Background(), &AgentGroup{
		AgentIdentifier: "my-agent",
		OrganizationId:  "org-id",
		EnvironmentId:   "env-id",
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", creds.Host)
}
