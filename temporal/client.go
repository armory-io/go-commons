package temporal

import (
	"crypto/tls"
	"fmt"
	"github.com/armory-io/go-commons/metrics"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	defaultNamespace             = "default"
	defaultHostname              = "localhost"
	defaultPort                  = "7233"
	armoryTemporalCloudAccountID = "88dfd"
)

type Configuration struct {
	Namespace                   string
	Hostname                    string
	Port                        string
	CertPath                    string
	KeyPath                     string
	TemporalCloudEnabled        bool
	TemporalCloudAccountID      string
	ClientSideEncryptionEnabled bool
	ClientSideEncryptionCMKARNs string
}

var Module = fx.Module(
	"temporal",
	fx.Provide(ArmoryTemporalProvider),
)

func ArmoryTemporalProvider(logger *zap.SugaredLogger, settings Configuration, metricsService *metrics.Metrics) (client.Client, error) {
	adapted := NewZapAdapter(logger.Desugar())
	options, err := optionsFromSettings(adapted, settings)
	if err != nil {
		return nil, err
	}
	options.MetricsHandler = newMetricsHandler(metricsService)
	return client.NewClient(*options)
}

func optionsFromSettings(logger *ZapAdapter, settings Configuration) (*client.Options, error) {
	if settings.TemporalCloudEnabled {
		if err := validateCloudConfig(settings); err != nil {
			return nil, err
		}
		return temporalCloudClientOptions(logger, settings)
	} else {
		return temporalClientOptions(logger, settings)
	}
}

func temporalClientOptions(logger *ZapAdapter, config Configuration) (*client.Options, error) {
	options := &client.Options{
		HostPort:           fmt.Sprintf("%s:%s", orDefault(config.Hostname, defaultHostname), orDefault(config.Port, defaultPort)),
		Logger:             logger,
		Namespace:          config.Namespace,
		ContextPropagators: []workflow.ContextPropagator{NewLoggerContextPropagator()},
	}

	if config.ClientSideEncryptionEnabled {
		options.DataConverter = NewEncryptionDataConverter(logger, converter.GetDefaultDataConverter(), EncryptionDataConverterOptions{CMKARNs: config.ClientSideEncryptionCMKARNs})
	}

	return options, nil
}

func validateCloudConfig(config Configuration) error {
	if config.KeyPath == "" {
		return fmt.Errorf("no client key path provided")
	}
	if config.CertPath == "" {
		return fmt.Errorf("no client cert path provided")
	}
	if config.ClientSideEncryptionEnabled && config.ClientSideEncryptionCMKARNs == "" {
		return fmt.Errorf("no cmk arns provided")
	}
	return nil
}

func temporalCloudClientOptions(logger *ZapAdapter, config Configuration) (*client.Options, error) {
	clientCertificate, err := tls.LoadX509KeyPair(config.CertPath, config.KeyPath)
	if err != nil {
		return nil, err
	}

	namespace := orDefault(config.Namespace, defaultNamespace)
	accountID := orDefault(config.TemporalCloudAccountID, armoryTemporalCloudAccountID)
	namespaceID := fmt.Sprintf("%s.%s", namespace, accountID)
	gRPCEndpoint := fmt.Sprintf("%s.%s.tmprl.cloud:7233", namespace, accountID)
	serverName := fmt.Sprintf("%s.%s.tmprl.cloud", namespace, accountID)

	options := &client.Options{
		Logger:    logger,
		HostPort:  gRPCEndpoint,
		Namespace: namespaceID,
		ConnectionOptions: client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{clientCertificate},
				ServerName:   serverName,
			},
		},
		ContextPropagators: []workflow.ContextPropagator{NewLoggerContextPropagator()},
	}

	if config.ClientSideEncryptionEnabled {
		options.DataConverter = NewEncryptionDataConverter(logger, converter.GetDefaultDataConverter(), EncryptionDataConverterOptions{CMKARNs: config.ClientSideEncryptionCMKARNs})
	}

	return options, nil
}

func orDefault(first, second string) string {
	if first != "" {
		return first
	}
	return second
}
