package iam

import "github.com/armory-io/lib-go-armory-cloud-commons/iam/token"

type Settings struct {
	JWT            JWT            `yaml:"jwt"`
	RequiredScopes []string       `yaml:"requiredScopes"`
	Identity       token.Identity `yaml:"identity"`
}

type JWT struct {
	JWTKeysURL string `yaml:"jwtKeysUrl"`
}
