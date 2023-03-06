package s3

import (
	"context"
	"github.com/armory-io/go-commons/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Configuration struct {
	Region              string
	Endpoint            string
	MaxRetryAttempts    int
	credentialsProvider aws.CredentialsProvider
}

func New(
	ctx context.Context,
	c Configuration,
	l *zap.SugaredLogger,
) (*s3.Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		endpoint := c.Endpoint
		if endpoint != "" {
			return aws.Endpoint{
				URL:               endpoint,
				PartitionID:       "aws",
				HostnameImmutable: true,
				SigningRegion:     region,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(c.Region),
		config.WithLogger(logging.AwsLoggerFromZapLogger(l, lo.ToPtr("s3"))),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithRetryMaxAttempts(c.MaxRetryAttempts),
	}

	if c.credentialsProvider != nil {
		opts = append(opts, config.WithCredentialsProvider(c.credentialsProvider))
	}

	ac, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(ac), nil
}
