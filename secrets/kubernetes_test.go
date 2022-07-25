package secrets

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewKubernetesSecretDecrypter(t *testing.T) {
	cases := []struct {
		in  string
		err string
	}{
		{
			in:  "blah",
			err: K8sGenericMalformedKeyError,
		},
		{
			in:  "!s:foo",
			err: K8sGenericMalformedKeyError,
		},
		{
			in:  "k:secret-key",
			err: K8sSecretNameMissingError,
		},
		{
			in:  "n:secret-key",
			err: K8sSecretKeyMissingError,
		},
		{
			in:  "n:kubernetes-secret-name!k:secret-key",
			err: "failed to determine namespace, you must supply the `!ns:` key or be running on a pod where /var/run/secrets/kubernetes.io/serviceaccount/namespace is defined",
		},
		{
			in:  "ns:foo!n:kubernetes-secret-name!k:secret-key",
			err: "",
		},
		{
			in:  "ns:foo!n:kubernetes-secret-name!k:secret-key!dne:bar",
			err: K8sGenericMalformedKeyError,
		},
	}

	for _, c := range cases {
		_, err := NewKubernetesSecretDecrypter(context.TODO(), false, c.in)
		eMsg := ""
		if err != nil {
			eMsg = err.Error()
		}
		assert.Equal(t, c.err, eMsg)
	}
}
