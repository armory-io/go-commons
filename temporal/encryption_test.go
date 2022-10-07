package temporal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"go.uber.org/zap"
	"strings"
	"testing"
)

func TestEncrypter(t *testing.T) {
	e := &Encrypter{
		CMKARNs: "arn",
		logger:  NewZapAdapter(zap.New(nil)),
		vc:      testVoynich{},
	}

	p := &commonpb.Payload{Data: []byte("hello from the other side")}
	assert.NoError(t, e.Encode(p))

	assert.Contains(t, string(p.Data), "encrypted:")
	assert.Equal(t, metadataEncodingEncrypted, string(p.Metadata[converter.MetadataEncoding]))

	assert.NoError(t, e.Decode(p))
	assert.NotContains(t, string(p.Data), "encrypted:")
}

type testVoynich struct{}

func (t testVoynich) Encrypt(bytes []byte, arns []string, contextKey, contextValue string) ([]byte, error) {
	return []byte(fmt.Sprintf("encrypted:%s", string(bytes))), nil
}

func (t testVoynich) Decrypt(bytes []byte, contextKey, contextValue string) ([]byte, error) {
	return []byte(strings.TrimPrefix(string(bytes), "encrypted:")), nil
}
