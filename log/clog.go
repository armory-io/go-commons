package log

import (
	"context"
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/temporal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

// NewClog creates a new Clog.
func NewClog(ctx temporal.LoggingValuer) Clog {
	if _, ok := ctx.(context.Context); ok {
		details, err := server.ExtractRequestDetailsFromContext(ctx.(context.Context))
		if err != nil {
			return Clog{
				logger: activity.GetLogger(ctx.(context.Context)),
			}
		}
		return Clog{
			logger: temporal.NewZapAdapter(details.LoggingMetadata.Logger.Desugar()),
		}
	} else if _, ok := ctx.(workflow.Context); ok {
		return Clog{
			logger: workflow.GetLogger(ctx.(workflow.Context)),
		}
	}
	return Clog{
		logger: NewNopLogger(),
	}
}

// Logger is a simplified abstraction of a Logger
type Logger interface {
	Debug(msg string, keyvals ...interface{})
	Info(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
}

// Clog delegates all calls to the underlying Logger
// is the default logging wrapper that can create
// logger instances
type Clog struct {
	logger log.Logger
}

// Debug logs a debug msg
func (l Clog) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, args)
}

// Info logs an info msg
func (l Clog) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, args)
}

// Error logs an error msg
func (l Clog) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, args)
}

// Warn logs a warn msg
func (l Clog) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, args)
}
