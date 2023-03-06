package s3

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewS3Client(t *testing.T) {
	l := zap.S()
	ctx := context.Background()

	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/my-bucket/path/to/blob", request.URL.Path)
		writer.WriteHeader(http.StatusOK)
	}))

	client, err := New(ctx, Configuration{
		Region:              "us-west-1",
		Endpoint:            s.URL,
		MaxRetryAttempts:    1,
		credentialsProvider: aws.AnonymousCredentials{},
	}, l)
	assert.NoError(t, err)

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: lo.ToPtr("my-bucket"),
		Key:    lo.ToPtr("path/to/blob"),
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
