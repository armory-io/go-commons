package scopes

import (
	"errors"
	"fmt"
	"strings"
)

type (
	Type       string
	Resource   string
	Permission string
)

const (
	ScopeDelimiter = ":"

	TypeAPI         Type = "api"
	TypeTargetGroup Type = "targetGroup"
	TypeAccount     Type = "account"

	ResourceDeployment   Resource = "deployment"
	ResourceTenant       Resource = "tenant"
	ResourceOrganization Resource = "organization"
	ResourceAgentHub     Resource = "agentHub"
	ResourceStar         Resource = "*"

	PermissionFull Permission = "full"
)

var (
	ScopeOrganizationAdmin     = mustScope(TypeAPI, ResourceOrganization, PermissionFull)
	ScopeTenantAdmin           = mustScope(TypeAPI, ResourceTenant, PermissionFull)
	ScopeDeploymentsFullAccess = mustScope(TypeAPI, ResourceDeployment, PermissionFull)
	ScopeRemoteNetworkAgent    = mustScope(TypeAPI, ResourceAgentHub, PermissionFull)

	types       = []Type{TypeAPI, TypeAccount, TypeTargetGroup}
	permissions = []Permission{PermissionFull}
)

var (
	ErrInvalidScope = errors.New("invalid scope")
	ErrInvalidGrant = errors.New("invalid grant")
)

type Grant struct {
	Type       Type
	Resource   Resource
	Permission Permission
}

func GrantFromStrings(t, r, p string) Grant {
	return Grant{Type: Type(t), Resource: Resource(r), Permission: Permission(p)}
}

func Parse(scope string) (Grant, error) {
	parts := strings.Split(scope, ScopeDelimiter)
	if len(parts) != 3 {
		return Grant{}, fmt.Errorf("%w: wanted 3 parts, got %d", ErrInvalidScope, len(parts))
	}

	t := Type(parts[0])
	r := Resource(parts[1])
	p := Permission(parts[2])

	v := validator{baseError: ErrInvalidScope}
	if err := v.validate(t, r, p); err != nil {
		return Grant{}, err
	}

	return Grant{
		Type:       t,
		Resource:   r,
		Permission: p,
	}, nil
}

func FromGrant(g Grant) (string, error) {
	t := g.Type
	r := g.Resource
	p := g.Permission

	v := validator{baseError: ErrInvalidGrant}

	if err := v.validate(t, r, p); err != nil {
		return "", err
	}

	return string(t) + ScopeDelimiter + string(r) + ScopeDelimiter + string(p), nil
}

type validator struct {
	baseError error
}

func (v validator) validate(t Type, r Resource, p Permission) error {
	if err := v.validateType(t); err != nil {
		return err
	}

	if err := v.validatePermission(p); err != nil {
		return err
	}

	switch t {
	case TypeAPI:
		if err := v.validateResourceForAPIType(r); err != nil {
			return err
		}
	case TypeTargetGroup:
		if err := v.validateResourceForTargetGroupType(r); err != nil {
			return err
		}
	}
	return nil
}

func (v validator) validateType(t Type) error {
	if !oneOf(types, t) {
		return fmt.Errorf("%w: unexpected type %q", v.baseError, t)
	}
	return nil
}

func (v validator) validatePermission(p Permission) error {
	if !oneOf(permissions, p) {
		return fmt.Errorf("%w: unexpected permission %q", v.baseError, p)
	}
	return nil
}

func (v validator) validateResourceForAPIType(r Resource) error {
	if !oneOf([]Resource{ResourceDeployment, ResourceTenant, ResourceOrganization, ResourceAgentHub}, r) {
		return fmt.Errorf("%w: invalid resource for api type: %q", v.baseError, r)
	}
	return nil
}

func (v validator) validateResourceForTargetGroupType(r Resource) error {
	if r == ResourceStar {
		return fmt.Errorf("%w: cannot use wildcard resource for targetGroup type", v.baseError)
	}
	return nil
}

func oneOf[T comparable](group []T, test T) bool {
	for _, g := range group {
		if test == g {
			return true
		}
	}
	return false
}

func mustScope(t Type, r Resource, p Permission) string {
	scope, err := FromGrant(Grant{t, r, p})
	if err != nil {
		panic(err)
	}
	return scope
}
