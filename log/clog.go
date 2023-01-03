package log

import (
	"github.com/armory-io/go-commons/server"
	"go.uber.org/zap"
)

type LoggerValuer interface {
	Value(any) any
}

// NewClog creates a new Clog.
func NewClog(ctx LoggerValuer) *zap.SugaredLogger {
	details, err := server.ExtractRequestDetailsFromContext(ctx)
	if err != nil {
		return zap.S()
	}
	return details.LoggingMetadata.Logger.With(server.ExtractLoggingFields(details.LoggingMetadata.Metadata)...)
}
