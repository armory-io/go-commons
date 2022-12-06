package yeti

import (
	"context"
	"encoding/json"
	"fmt"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/iam/token"
	"go.uber.org/zap"
	"net/http"
)

type (
	ClientConfiguration struct {
		Rest armoryhttp.ClientSettings
	}

	Client struct {
		http *armoryhttp.Client
	}

	Role struct {
		OrgID string  `json:"orgID"`
		EnvID *string `json:"envID"`
		Name  string  `json:"name"`
	}
)

func NewClient(ctx context.Context, log *zap.SugaredLogger, config ClientConfiguration, identity token.Identity) (*Client, error) {
	h, err := armoryhttp.NewClient(ctx, log, config.Rest, identity)
	if err != nil {
		return nil, err
	}

	return &Client{http: h}, nil
}

func (yc *Client) FetchUserRoles(orgID, envID, userID string) ([]Role, error) {
	req, err := yc.http.NewRequest(http.MethodGet, fmt.Sprintf("/organizations/%s/environments/%s/users/%s/roles", orgID, envID, userID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := yc.http.GetClient().Do(req)
	if err != nil {
		return nil, err
	}

	var roles []Role
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}

	return roles, nil
}
