/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	//  PrincipalType The type of principal, user or machine
	Type PrincipalType `json:"type"`
	// Name  This is the principals name For user types this is will the users email address For machine  types this will be the identifier of the OIDC application that represents the machine
	Name string `json:"name"`
	// OrgId The guid for the organization the principal is a member of
	OrgId string `json:"orgId"`
	// OrgName The human-readable name of the organization
	OrgName string `json:"orgName"`
	// EnvId The guid for the environment (aka tenant) that this principal is authorized for
	EnvId string `json:"envId"`
	// ArmoryAdmin A flag to determine if the principal is an armory admin principal and can do dangerous x-org and or x-env actions.
	ArmoryAdmin bool `json:"armoryAdmin"`
	// Subject  The "sub" (subject) claim identifies the principal that is the subject of the JWT.  The "sub" value is a case-sensitive string containing a StringOrURI value.
	Subject string `json:"sub"`
	/// Issuer The "iss" (issuer) claim identifies the principal that issued the JWT.
	Issuer string `json:"iss"`
	// AuthorizedParty OPTIONAL. Authorized party - the party to which the ID Token was issued. If present, it MUST contain the OAuth 2.0 Client ID of this party. This Claim is only needed when the ID Token has
	// a single audience value and that audience is different from the authorized party. It MAY be included even when the authorized party is the same as the sole audience. The azp value is a case-sensitive string containing a StringOrURI value.
	AuthorizedParty string `json:"azp"`
	// Scopes A list of scopes that was set by the authorization server such as use:adminApi
	Scopes []string `json:"scopes"`
	// Roles List of groups that a principal belongs to
	Roles []string `json:"roles"`

	UserId string `json:"userId"`
}

func (p *ArmoryCloudPrincipal) Tenant() string {
	return fmt.Sprintf("%s:%s", p.OrgId, p.EnvId)
}

func (p *ArmoryCloudPrincipal) String() string {
	return fmt.Sprintf("Principal: %s, Type: %s, OrgId: %s, EnvId: %s, Scopes: %s",
		p.Name, p.Type, p.OrgId, p.EnvId, strings.Join(p.Scopes, ", "))
}

func (p *ArmoryCloudPrincipal) UnsafeHasScope(scope string) bool {
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

func (p *ArmoryCloudPrincipal) HasScope(scope string) bool {
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
