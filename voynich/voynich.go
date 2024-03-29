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

package voynich

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	voynichHeaderCMKARNs      = "x-voynich-cmk-arns"
	voynichHeaderContextKey   = "x-voynich-context-key"
	voynichHeaderContextValue = "x-voynich-context-value"
)

type Client struct {
	Host       string
	Port       string
	HTTPClient *http.Client
}

func New() *Client {
	return &Client{
		Host:       "127.0.0.1",
		Port:       "1404",
		HTTPClient: http.DefaultClient,
	}
}

type request struct {
	ContextKey   string
	ContextValue string
	CMKARNs      []string
	Data         []byte
}

func (c *Client) Decrypt(b []byte, contextKey, contextValue string) ([]byte, error) {
	return c.callVoynich(&request{
		ContextKey:   contextKey,
		ContextValue: contextValue,
		Data:         b,
	}, "/decrypt")
}

func (c *Client) Encrypt(b []byte, cmkARNs []string, contextKey, contextValue string) ([]byte, error) {
	if len(cmkARNs) == 0 {
		return nil, fmt.Errorf("must provide CMK ARN")
	}

	return c.callVoynich(&request{
		ContextKey:   contextKey,
		ContextValue: contextValue,
		CMKARNs:      cmkARNs,
		Data:         b,
	}, "/encrypt")
}

func (c *Client) callVoynich(vReq *request, path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%s%s", c.Host, c.Port, path), bytes.NewReader(vReq.Data))
	if err != nil {
		return nil, err
	}

	if len(vReq.CMKARNs) != 0 {
		req.Header.Add(voynichHeaderCMKARNs, strings.Join(vReq.CMKARNs, ","))
	}
	req.Header.Add(voynichHeaderContextKey, vReq.ContextKey)
	req.Header.Add(voynichHeaderContextValue, vReq.ContextValue)
	req.Header.Add("Content-Type", "application/octet-stream")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not encode data, status=%d, message=%s", res.StatusCode, string(body))
	}

	return body, nil
}
