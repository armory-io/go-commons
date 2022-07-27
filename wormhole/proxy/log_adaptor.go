package proxy

import "go.uber.org/zap"

type logAdapter struct {
	*zap.SugaredLogger
}

func (la *logAdapter) Printf(msg string, args ...any) {
	la.Debugf(msg, args...)
}
