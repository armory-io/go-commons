package graphql

import (
	"context"
	"encoding/json"
	"github.com/Khan/genqlient/graphql"
	"github.com/armory-io/go-commons/iam"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientRequest(t *testing.T) {
	orgID := uuid.NewString()
	envID := uuid.NewString()

	ctx := iam.WithPrincipal(context.Background(), iam.ArmoryCloudPrincipal{
		OrgId: orgID,
		EnvId: envID,
		Name:  "principal-or-principle?",
	})

	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "admin", request.Header.Get(adminSecretHeader))
		assert.Equal(t, orgID, request.Header.Get(orgIDHeader))
		assert.Equal(t, envID, request.Header.Get(envIDHeader))
		assert.Equal(t, "principal-or-principle?", request.Header.Get(principalNameHeader))
		assert.Equal(t, superuserRole, request.Header.Get(roleHeader))

		_, err := writer.Write(lo.Must(json.Marshal(map[string]any{
			"data": map[string]any{
				"insertPipelinesOne": map[string]any{
					"id": "bananas",
				},
			},
		})))
		assert.NoError(t, err)
	}))

	c := NewClient(Configuration{
		BaseURL:     s.URL,
		AdminSecret: "admin",
	}, http.DefaultClient)

	var response graphql.Response
	assert.NoError(t, c.MakeRequest(ctx, &graphql.Request{}, &response))
	assert.NotNil(t, response.Data)
}

func TestNewClientNoPrincipal(t *testing.T) {
	c := NewClient(Configuration{
		BaseURL:     "should-never-be-called",
		AdminSecret: "admin",
	}, http.DefaultClient)

	err := c.MakeRequest(context.Background(), &graphql.Request{}, &graphql.Response{})
	assert.ErrorIs(t, err, ErrUserPrincipalNotFound)
}
