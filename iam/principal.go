package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type PrincipalType string

const (
	User    PrincipalType = "user"
	Machine PrincipalType = "machine"
)

type ArmoryCloudPrincipal struct {
	Type        PrincipalType `json:"type"`
	Name        string        `json:"name"`
	OrgId       string        `json:"orgId"`
	OrgName     string        `json:"orgName"`
	EnvId       string        `json:"envId"`
	ArmoryAdmin bool          `json:"armoryAdmin"`
	Scopes      []string      `json:"scopes"`
	Groups      []string      `json:"groups"`
}

func (p *ArmoryCloudPrincipal) Tenant() string {
	return fmt.Sprintf("%s:%s", p.OrgId, p.EnvId)
}

func (p *ArmoryCloudPrincipal) String() string {
	return fmt.Sprintf("Principal: %s, Type: %s, OrgId: %s, EnvId: %s, Scopes: %s",
		p.Name, p.Type, p.OrgId, p.EnvId, strings.Join(p.Scopes, ", "))
}

func (p *ArmoryCloudPrincipal) HasScope(scope string) bool {
	// allow users to do everything until proper RBAC is in place
	if p.Type == User {
		return true
	}
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (p *ArmoryCloudPrincipal) ToJson() string {
	payload, _ := json.Marshal(p)
	return string(payload)
}

func WithPrincipal(ctx context.Context, principal ArmoryCloudPrincipal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}
