package logging

import (
	"go.uber.org/zap"
)

type Settings struct {
}

func New(settings Settings) (*zap.SugaredLogger, error) {
	log, err := zap.NewProduction(
		zap.WithCaller(true),
	)
	if err != nil {
		return nil, err
	}
	return log.Sugar(), nil
}
