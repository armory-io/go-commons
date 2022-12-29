package log

import (
	"context"
	"github.com/armory-io/go-commons/server"
	"github.com/armory-io/go-commons/temporal"
	"github.com/samber/lo"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	defaultLogger "log"
)

// NewClog creates a new Clog.
func NewClog(ctx temporal.LoggingValuer) Clog {
	if _, ok := ctx.(context.Context); ok {
		details, err := server.ExtractRequestDetailsFromContext(ctx.(context.Context))
		if err != nil {
			return Clog{}
		}
		return Clog{
			zlogger: details.LoggingMetadata.Logger,
		}
	} else if _, ok := ctx.(workflow.Context); ok {
		return Clog{
			tlogger: workflow.GetLogger(ctx.(workflow.Context)),
		}
	}
	return Clog{}
}

// Logger is a simplified abstraction of a Logger
type Logger interface {
	Info(msg string)
	Error(msg string)
	Warn(msg string)
}

// Clog delegates all calls to the underlying Logger
// is the default logging wrapper that can create
// logger instances
type Clog struct {
	zlogger *zap.SugaredLogger
	tlogger log.Logger
}

// Info logs an info msg
func (l Clog) Info(msg string) {
	if l.zlogger != nil {
		l.zlogger.Info(msg)
	} else if lo.IsNotEmpty(l.tlogger) {
		l.tlogger.Info(msg)
	} else {
		defaultLogger.Println(msg)
	}
}

// Error logs an error msg
func (l Clog) Error(msg string) {
	if l.zlogger != nil {
		l.zlogger.Error(msg)
	} else if lo.IsNotEmpty(l.tlogger) {
		l.tlogger.Error(msg)
	} else {
		defaultLogger.Println(msg)
	}
}

// Warn logs a warn msg
func (l Clog) Warn(msg string) {
	if l.zlogger != nil {
		l.zlogger.Warn(msg)
	} else if lo.IsNotEmpty(l.tlogger) {
		l.tlogger.Warn(msg)
	} else {
		defaultLogger.Println(msg)
	}
}
