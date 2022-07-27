/*
 * Copyright (c) 2022 Armory, Inc
 *   National Electronics and Computer Technology Center, Thailand
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package token

type Identity struct {
	Token                   string      `yaml:"token,omitempty" json:"token,omitempty"`
	TokenCommand            *Command    `yaml:"tokenCommand,omitempty" json:"tokenCommand,omitempty"`
	Armory                  ArmoryCloud `yaml:"armory,omitempty" json:"armory,omitempty"`
	RefreshIntervalSeconds  int64       `yaml:"refreshIntervalSeconds" json:"refreshIntervalSeconds"`
	ExpirationLeewaySeconds int64       `yaml:"expirationLeewaySeconds" json:"expirationLeewaySeconds"`
}

type Command struct {
	Command string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
}

type ArmoryCloud struct {
	ClientId       string `yaml:"clientId,omitempty" json:"clientId,omitempty"`
	Secret         string `yaml:"secret,omitempty" json:"secret,omitempty"`
	TokenIssuerUrl string `yaml:"tokenIssuerUrl,omitempty" json:"tokenIssuerUrl,omitempty"`
	Audience       string `yaml:"audience,omitempty" json:"audience,omitempty"`
	Verify         bool   `yaml:"verify" json:"verify"`
}

func DefaultArmoryCloud() ArmoryCloud {
	return ArmoryCloud{
		TokenIssuerUrl: "https://auth.cloud.armory.io/oauth/token",
		Audience:       "https://api.cloud.armory.io",
		Verify:         true,
	}
}

func DefaultIdentity() Identity {
	return Identity{
		Armory:                  DefaultArmoryCloud(),
		ExpirationLeewaySeconds: 30,
	}
}
