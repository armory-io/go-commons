package iam

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTokenToPrincipal(t *testing.T) {
	token := map[string]any{
		"name":        "frankie",
		"type":        "user",
		"orgId":       "org-id",
		"orgName":     "dogz that deploy",
		"envId":       "env-id",
		"armoryAdmin": false,
		"scopes": []string{
			"api:organization:full",
		},
		"roles": []string{
			"Org Admin",
		},
		"iss": "https://test.issuer/",
		"sub": "test_subject_123",
		"azp": "authorized party",
	}
	scopes := "openid profile email"
	principal, err := tokenToPrincipal(token, scopes)
	assert.NoError(t, err)
	assert.Equal(t, ArmoryCloudPrincipal{
		Name:            "frankie",
		Type:            User,
		OrgId:           "org-id",
		OrgName:         "dogz that deploy",
		EnvId:           "env-id",
		ArmoryAdmin:     false,
		Subject:         "test_subject_123",
		Issuer:          "https://test.issuer/",
		AuthorizedParty: lo.ToPtr("authorized party"),
		Scopes: []string{
			"api:organization:full",
			"openid",
			"profile",
			"email",
		},
		Roles: []string{
			"Org Admin",
		},
	}, *principal)
}
