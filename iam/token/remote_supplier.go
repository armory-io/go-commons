package token

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lestrrat-go/jwx/jwt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func newRemoteTokenSupplier(cloud ArmoryCloud) *remoteTokenSupplier {
	return &remoteTokenSupplier{
		settings: cloud,
	}
}

type remoteTokenSupplier struct {
	settings ArmoryCloud
}

func (r *remoteTokenSupplier) GetToken(ctx context.Context) (string, *time.Time, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", r.settings.ClientId)
	data.Set("client_secret", r.settings.Secret)
	data.Set("audience", r.settings.Audience)
	req, err := http.NewRequest(http.MethodPost, r.settings.TokenIssuerUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("accept", "application/json")
	c := &http.Client{}
	res, err := c.Do(req)
	if err != nil {
		return "", nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", nil, fmt.Errorf("unexpected status code while getting token %d", res.StatusCode)
	}
	defer res.Body.Close()
	tk, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil, err
	}

	rt := &remoteToken{}
	if err := json.Unmarshal(tk, rt); err != nil {
		return "", nil, fmt.Errorf("unable to parse response from %s: %w", r.settings.TokenIssuerUrl, err)
	}
	if rt.AccessToken == "" {
		return "", nil, fmt.Errorf("no access_token returned from %s", r.settings.TokenIssuerUrl)
	}

	t, err := jwt.Parse([]byte(rt.AccessToken), r.parseOptions()...)
	if err != nil {
		return "", nil, err
	}
	exp := t.Expiration()
	return rt.AccessToken, &exp, nil
}

func (r *remoteTokenSupplier) parseOptions() []jwt.ParseOption {
	var opts []jwt.ParseOption
	if r.settings.Verify {
		opts = append(opts, jwt.WithValidate(true))
	}
	return opts
}

type remoteToken struct {
	AccessToken string `json:"access_token"`
}
