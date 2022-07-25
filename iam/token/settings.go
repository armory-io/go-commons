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
