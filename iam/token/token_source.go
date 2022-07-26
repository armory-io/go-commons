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
