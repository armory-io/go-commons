package token

import (
	"context"
	"go.uber.org/zap"
	"time"
)

func GetCredentials(auth Identity, log *zap.SugaredLogger) *refreshableTokenCredentials {
	getter := getTokenGetter(auth, log)
	if getter == nil {
		log.Warn("no token supplier specified. Use auth.identity.armory")
		return nil
	}

	log.Info("token expiration will be detected")
	return newRefreshableTokenCredentials(getter, auth.ExpirationLeewaySeconds)
}

type tokenGetter interface {
	GetToken(ctx context.Context) (string, *time.Time, error)
}

func getTokenGetter(auth Identity, log *zap.SugaredLogger) tokenGetter {
	if auth.Armory.ClientId != "" {
		log.Infof("set to obtain token from %s", auth.Armory.TokenIssuerUrl)
		return newRemoteTokenSupplier(auth.Armory)
	}
	return nil
}
