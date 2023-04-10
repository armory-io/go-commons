package temporal

import (
	"crypto/tls"
	"fmt"
	"github.com/armory-io/go-commons/metrics"
	"github.com/armory-io/go-commons/opentelemetry"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
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
	fx.Provide(ClientProvider),
	fx.Provide(WorkerProviderProvider),
)

type ProviderParameters struct {
	fx.In

	Logger         *zap.SugaredLogger
	Config         Configuration
	MetricsService metrics.MetricsSvc
	Tracing        opentelemetry.Configuration `optional:"true"`
}

func ClientProvider(params ProviderParameters) (client.Client, error) {
	adapted := NewZapAdapter(params.Logger.Desugar())
	options, err := optionsFromParams(adapted, params)
	if err != nil {
		return nil, err
	}
	options.MetricsHandler = newMetricsHandler(params.MetricsService)
	return client.Dial(*options)
}

func optionsFromParams(logger *ZapAdapter, params ProviderParameters) (*client.Options, error) {
	if params.Config.TemporalCloudEnabled {
		if err := validateCloudConfig(params.Config); err != nil {
			return nil, err
		}
		return temporalCloudClientOptions(logger, params)
	} else {
		return temporalClientOptions(logger, params)
	}
}

func temporalClientOptions(logger *ZapAdapter, params ProviderParameters) (*client.Options, error) {
	config := params.Config

	var interceptors []interceptor.ClientInterceptor
	if params.Tracing.Push.Enabled {
		otelInterceptor, err := newOtelInterceptor()
		if err != nil {
			return nil, err
		}
		interceptors = append(interceptors, otelInterceptor)
	}

	options := &client.Options{
		HostPort:           fmt.Sprintf("%s:%s", orDefault(config.Hostname, defaultHostname), orDefault(config.Port, defaultPort)),
		Logger:             logger,
		Namespace:          config.Namespace,
		ContextPropagators: []workflow.ContextPropagator{NewLoggerContextPropagator(), newWorkflowObservabilityParametersPropagator()},
		Interceptors:       interceptors,
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

func temporalCloudClientOptions(logger *ZapAdapter, params ProviderParameters) (*client.Options, error) {
	config := params.Config
	clientCertificate, err := tls.LoadX509KeyPair(config.CertPath, config.KeyPath)
	if err != nil {
		return nil, err
	}

	var interceptors []interceptor.ClientInterceptor
	if params.Tracing.Push.Enabled {
		otelInterceptor, err := newOtelInterceptor()
		if err != nil {
			return nil, err
		}
		interceptors = append(interceptors, otelInterceptor)
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
		ContextPropagators: []workflow.ContextPropagator{NewLoggerContextPropagator(), newWorkflowObservabilityParametersPropagator()},
		Interceptors:       interceptors,
	}

	return options, nil
}

func orDefault(first, second string) string {
	if first != "" {
		return first
	}
	return second
}
