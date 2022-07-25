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
