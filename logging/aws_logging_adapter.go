package logging

import (
	"fmt"
	"github.com/aws/smithy-go/logging"
	"go.uber.org/zap"
)

func AwsLoggerFromZapLogger(logger *zap.SugaredLogger, prefix *string) logging.LoggerFunc {
	var awsLogger logging.LoggerFunc = func(classification logging.Classification, format string, v ...interface{}) {
		header := format
		if prefix != nil {
			header = fmt.Sprintf("%s:%s", *prefix, format)
		}
		switch classification {
		case logging.Debug:
			logger.Debugf(header, v...)
		case logging.Warn:
			logger.Warnf(header, v...)
		default:
			logger.Infof(header, v...)
		}
	}
	return awsLogger
}
