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
