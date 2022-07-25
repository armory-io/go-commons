package secrets

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNoop(t *testing.T) {
	noop, err := NewNoopDecrypter(context.TODO(), false, "mynotsosecretstring")
	if assert.Nil(t, err) {
		s, err := noop.Decrypt()
		assert.Nil(t, err)
		assert.Equal(t, "mynotsosecretstring", s)
	}
}