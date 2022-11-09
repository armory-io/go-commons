package logging

import (
	"github.com/aws/smithy-go/logging"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
)

func TestAwsLoggerAdapter(t *testing.T) {

	var logs []zapcore.Entry

	option := zap.Hooks(func(entry zapcore.Entry) error {
		logs = append(logs, entry)
		return nil
	})
	coreLogger, err := createArmoryConsoleLogger([]zap.Option{option}, zapcore.DebugLevel)
	if err != nil {
		t.Fatal(err)
	}
	logger := coreLogger.Sugar()

	tests := []struct {
		name     string
		testCase func(t *testing.T)
	}{
		{
			name: "no prefix, debug level",
			testCase: func(t *testing.T) {
				victim := AwsLoggerFromZapLogger(logger, nil)
				victim.Logf(logging.Debug, "%d + %.2f = %s", 2, 2.2, "5")

				assert.Equal(t, "2 + 2.20 = 5", logs[0].Message)
				assert.Equal(t, zapcore.DebugLevel, logs[0].Level)
			},
		},
		{
			name: "no prefix, warning level",
			testCase: func(t *testing.T) {
				victim := AwsLoggerFromZapLogger(logger, nil)
				victim.Logf(logging.Warn, "abort%s", "!")

				assert.Equal(t, "abort!", logs[0].Message)
				assert.Equal(t, zapcore.WarnLevel, logs[0].Level)
			},
		},
		{
			name: "no prefix, default level",
			testCase: func(t *testing.T) {
				victim := AwsLoggerFromZapLogger(logger, nil)
				victim.Logf("i don't know this one", "ok")

				assert.Equal(t, "ok", logs[0].Message)
				assert.Equal(t, zapcore.InfoLevel, logs[0].Level)
			},
		},
		{
			name: "s3 prefix, default level",
			testCase: func(t *testing.T) {
				victim := AwsLoggerFromZapLogger(logger, lo.ToPtr("[s3]"))
				victim.Logf(logging.Debug, "hello from %s", "s3")

				assert.Equal(t, "[s3]:hello from s3", logs[0].Message)
				assert.Equal(t, zapcore.DebugLevel, logs[0].Level)
			},
		},
	}

	for _, tc := range tests {
		logs = []zapcore.Entry{}
		t.Run(tc.name, tc.testCase)
	}
}
