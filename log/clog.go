package log

import (
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/temporal"
	"go.uber.org/zap"
)

func NewClog(ctx temporal.LoggingValuer) *zap.SugaredLogger {
	v, ok := ctx.Value(server.RequestDetailsKey{}).(server.RequestDetails)
	if !ok {
		return zap.S()
	}
	return v.LoggingMetadata.Logger.With(v.LoggingMetadata.Metadata...)
}
