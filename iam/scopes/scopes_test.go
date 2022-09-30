package scopes

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParse(t *testing.T) {
	valid := []pair[string, Grant]{
		pairFrom("api:organization:full", GrantFromStrings("api", "organization", "full")),
		pairFrom("api:tenant:full", GrantFromStrings("api", "tenant", "full")),
		pairFrom("api:deployment:full", GrantFromStrings("api", "deployment", "full")),
		pairFrom("api:agentHub:full", GrantFromStrings("api", "agentHub", "full")),
		pairFrom("targetGroup:potato-facts:full", GrantFromStrings("targetGroup", "potato-facts", "full")),
		pairFrom("account:eks-dev-cluster:full", GrantFromStrings("account", "eks-dev-cluster", "full")),
		pairFrom("account:*:full", GrantFromStrings("account", "*", "full")),
	}

	for _, v := range valid {
		t.Run(fmt.Sprintf("valid: %s", v.left), func(t *testing.T) {
			grant, err := Parse(v.left)
			assert.NoError(t, err)
			assert.Equal(t, v.right, grant)
		})
	}

	invalid := []string{
		"dachshund:deployment:full",
		"api:chihuahua:full",
		"api:deployment:pomeranian",
		"too:many:parts:here",
		"targetGroup:*:full",
		"api:*:full",
		"::",
		"api::full",
		":*:full",
		":*:",
	}

	for _, i := range invalid {
		t.Run(fmt.Sprintf("invalid: %s", i), func(t *testing.T) {
			_, err := Parse(i)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidScope)
		})
	}
}

func TestGrantFrom(t *testing.T) {
	valid := []pair[Grant, string]{
		pairFrom(GrantFromStrings("api", "organization", "full"), "api:organization:full"),
		pairFrom(GrantFromStrings("api", "tenant", "full"), "api:tenant:full"),
		pairFrom(GrantFromStrings("api", "deployment", "full"), "api:deployment:full"),
		pairFrom(GrantFromStrings("api", "agentHub", "full"), "api:agentHub:full"),
		pairFrom(GrantFromStrings("targetGroup", "potato-facts", "full"), "targetGroup:potato-facts:full"),
		pairFrom(GrantFromStrings("account", "eks-dev-cluster", "full"), "account:eks-dev-cluster:full"),
		pairFrom(GrantFromStrings("account", "*", "full"), "account:*:full"),
	}

	for _, v := range valid {
		t.Run(fmt.Sprintf("valid: %s", v.left), func(t *testing.T) {
			scope, err := FromGrant(v.left)
			assert.NoError(t, err)
			assert.Equal(t, v.right, scope)
		})
	}

	invalid := []Grant{
		GrantFromStrings("dachshund", "deployment", "full"),
		GrantFromStrings("api", "chihuahua", "full"),
		GrantFromStrings("api", "deployment", "pomeranian"),
		GrantFromStrings("targetGroup", "*", "full"),
		GrantFromStrings("api", "*", "full"),
		GrantFromStrings("", "", ""),
		GrantFromStrings("api", "", "full"),
		GrantFromStrings("", "*", "full"),
		GrantFromStrings("", "*", ""),
	}

	for _, i := range invalid {
		t.Run(fmt.Sprintf("invalid: %s", i), func(t *testing.T) {
			_, err := FromGrant(i)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidGrant)
		})
	}
}

type pair[T, U any] struct {
	left  T
	right U
}

func pairFrom[T, U any](left T, right U) pair[T, U] {
	return pair[T, U]{left, right}
}
