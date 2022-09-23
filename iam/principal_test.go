package iam

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHasScopes(t *testing.T) {
	user := ArmoryCloudPrincipal{
		Name:        "frankie",
		Type:        User,
		OrgId:       "org-id",
		OrgName:     "dogz that deploy",
		EnvId:       "env-id",
		ArmoryAdmin: false,
		Scopes: []string{
			"api:organization:full",
			"openid",
			"profile",
			"email",
		},
		Roles: []string{
			"Org Admin",
		},
	}

	machine := ArmoryCloudPrincipal{
		Name:        "robot-frankie",
		Type:        Machine,
		OrgId:       "org-id",
		OrgName:     "dogz that deploy",
		EnvId:       "env-id",
		ArmoryAdmin: false,
		Scopes: []string{
			"read:customerConfiguration",
			"openid",
			"profile",
			"email",
		},
	}

	assert.True(t, user.HasScope("openid"))
	assert.True(t, user.UnsafeHasScope("openid"))
	assert.False(t, user.HasScope("does:not:exist"))
	assert.True(t, user.UnsafeHasScope("does:not:exist"))

	assert.True(t, machine.HasScope("openid"))
	assert.True(t, machine.UnsafeHasScope("openid"))
	assert.False(t, machine.HasScope("does:not:exist"))
	assert.False(t, machine.UnsafeHasScope("does:not:exist"))
}
