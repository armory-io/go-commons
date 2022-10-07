package temporal

import (
	"github.com/armory-io/go-commons/voynich"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"strings"
)

const (
	metadataEncodingEncrypted           = "binary/encrypted"
	armoryKmsEncryptionContextKey       = "armory.io/encryption-context"
	temporalCloudEncryptionContextValue = "temporal-client-encryption"
)

type EncryptionDataConverter struct {
	converter.EncodingDataConverter
	options EncryptionDataConverterOptions
}

type EncryptionDataConverterOptions struct {
	CMKARNs string
}

func NewEncryptionDataConverter(logger *ZapAdapter, dataConverter converter.DataConverter, options EncryptionDataConverterOptions) *EncryptionDataConverter {
	return &EncryptionDataConverter{
		EncodingDataConverter: *converter.NewEncodingDataConverter(dataConverter, &Encrypter{
			CMKARNs: options.CMKARNs,
			vc:      voynich.New(),
			logger:  logger,
		}),
		options: options,
	}
}

type voynichClient interface {
	Encrypt(bytes []byte, arns []string, contextKey string, contextValue string) ([]byte, error)
	Decrypt(bytes []byte, contextKey string, contextValue string) ([]byte, error)
}

type Encrypter struct {
	CMKARNs string
	vc      voynichClient
	logger  *ZapAdapter
}

func (e *Encrypter) Encode(p *commonpb.Payload) (err error) {
	b, err := p.Marshal()
	if err != nil {
		return err
	}

	encrypted, err := e.vc.Encrypt(b, strings.Split(e.CMKARNs, ","), armoryKmsEncryptionContextKey, temporalCloudEncryptionContextValue)
	if err != nil {
		return err
	}

	p.Data = encrypted
	p.Metadata = map[string][]byte{
		converter.MetadataEncoding: []byte(metadataEncodingEncrypted),
	}

	return nil
}

func (e *Encrypter) Decode(p *commonpb.Payload) (err error) {
	if string(p.Metadata[converter.MetadataEncoding]) != metadataEncodingEncrypted {
		return nil
	}

	decrypted, err := e.vc.Decrypt(p.Data, armoryKmsEncryptionContextKey, temporalCloudEncryptionContextValue)
	if err != nil {
		return err
	}

	p.Reset()
	return p.Unmarshal(decrypted)
}
