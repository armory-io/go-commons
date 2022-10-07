package temporal

import (
	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"testing"
)

func TestExtractLoggerMetadata(t *testing.T) {
	header := &common.Header{
		Fields: map[string]*common.Payload{
			propagationKey: {
				Metadata: map[string][]byte{
					converter.MetadataEncoding: []byte(converter.MetadataEncodingJSON),
				},
				Data: []byte(`[{"key": "PipelineID", "value": "1-800-pipelines"}]`),
			},
		},
	}

	metadata, err := ExtractLoggerMetadata(header)
	assert.NoError(t, err)
	assert.Equal(t, "1-800-pipelines", metadata["PipelineID"])
}
