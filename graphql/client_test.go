package graphql

import (
	"context"
	"encoding/json"
	"github.com/armory-io/go-commons/iam"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

type (
	insertPipelinesOne struct {
		Pipeline struct {
			ID string
		} `graphql:"insertPipelinesOne(object: $object)"`
	}

	pipelinesInsertInput struct {
		Application applicationInsert `json:"application"`
		Description string            `json:"description"`
	}

	applicationInsert struct {
		Data       application `json:"data"`
		OnConflict onConflict  `json:"onConflict"`
	}

	application struct {
		Name string `json:"name"`
	}

	onConflict struct {
		Constraint    string   `json:"constraint"`
		UpdateColumns []string `json:"updateColumns"`
	}
)

func (p *pipelinesInsertInput) GetGraphQLType() string {
	return "pipelinesInsertInput"
}

func TestNewClientRequest(t *testing.T) {
	orgID := uuid.NewString()
	envID := uuid.NewString()

	ctx := iam.WithPrincipal(context.Background(), iam.ArmoryCloudPrincipal{
		OrgId: orgID,
		EnvId: envID,
	})

	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "admin", request.Header.Get(adminSecretHeader))
		assert.Equal(t, orgID, request.Header.Get(orgIDHeader))
		assert.Equal(t, envID, request.Header.Get(envIDHeader))
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

	var insertPipelinesOneResult insertPipelinesOne
	assert.NoError(t, c.Mutate(ctx, &insertPipelinesOneResult, map[string]any{
		"object": pipelinesInsertInput{
			Description: "great pipeline!",
			Application: applicationInsert{
				Data: application{
					Name: "deploy-engine",
				},
				OnConflict: onConflict{
					Constraint:    "key_applications_org_id_env_id_name",
					UpdateColumns: []string{"name"},
				},
			},
		},
	}))

	assert.Equal(t, "bananas", insertPipelinesOneResult.Pipeline.ID)
}

func TestNewClientNoPrincipal(t *testing.T) {
	c := NewClient(Configuration{
		BaseURL:     "should-never-be-called",
		AdminSecret: "admin",
	}, http.DefaultClient)

	var insertPipelinesOneResult insertPipelinesOne
	err := c.Mutate(context.Background(), &insertPipelinesOneResult, map[string]any{
		"object": pipelinesInsertInput{
			Description: "great pipeline!",
			Application: applicationInsert{
				Data: application{
					Name: "deploy-engine",
				},
				OnConflict: onConflict{
					Constraint:    "key_applications_org_id_env_id_name",
					UpdateColumns: []string{"name"},
				},
			},
		},
	})

	// The client unwraps our error type :(
	assert.ErrorContains(t, err, ErrUserPrincipalNotFound.Error())
}
