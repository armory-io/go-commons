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
	Type        PrincipalType `json:"type"`
	Name        string        `json:"name"`
	OrgId       string        `json:"orgId"`
	OrgName     string        `json:"orgName"`
	EnvId       string        `json:"envId"`
	ArmoryAdmin bool          `json:"armoryAdmin"`
	Scopes      []string      `json:"scopes"`
	Roles       []string      `json:"roles"`
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
