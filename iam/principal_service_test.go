package iam

import (
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockJwtFetcher struct {
	token interface{}
}

func (j *MockJwtFetcher) Download() error {
	return nil
}

func (j *MockJwtFetcher) Fetch(t []byte) (interface{}, interface{}, error) {
	token := map[string]interface{}{
		"name": string(t),
	}
	j.token = token
	return token, nil, nil
}

func TestArmoryCloudPrincipalMiddleware(test *testing.T) {
	type PrincipalServiceTest struct {
		desc       string
		fetcher    *MockJwtFetcher
		validators []PrincipalValidator
		headers    map[string]string
		statusCode int
		errorMsg   string
		assertion  func(t *testing.T, tc PrincipalServiceTest)
	}
	cases := []PrincipalServiceTest{
		{
			desc:    "happy path",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"Authorization": "Bearer ulice",
			},
			validators: []PrincipalValidator{
				func(p *ArmoryCloudPrincipal) error {
					return nil
				},
			},
			statusCode: http.StatusOK,
		},
		{
			desc:    "Missing Auth headers",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"good": "dobry",
			},
			statusCode: http.StatusUnauthorized,
			errorMsg:   "Must provide Authorization header",
		},
		{
			desc:    "bad Auth header",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"Authorization": "ulice",
			},
			statusCode: http.StatusUnauthorized,
			errorMsg:   "Malformed token",
		},
		{
			desc:    "should prioritize Glados proxied header",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"Authorization":                  "Bearer ulice",
				"X-Armory-Proxied-Authorization": "Bearer jezero",
			},
			assertion: func(t *testing.T, tc PrincipalServiceTest) {
				token := map[string]interface{}{
					"name": "jezero",
				}
				assert.Equal(t, token, tc.fetcher.token, "Tokens do not match")
			},
			statusCode: http.StatusOK,
		},
		{
			desc:    "should reject failed validators",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"Authorization": "Bearer ulice",
			},
			validators: []PrincipalValidator{
				func(p *ArmoryCloudPrincipal) error {
					return errors.New("failed validation")
				},
			},
			statusCode: http.StatusForbidden,
			errorMsg:   "failed validation",
		},
		{
			desc:    "should run multiple validators",
			fetcher: &MockJwtFetcher{},
			headers: map[string]string{
				"Authorization": "Bearer ulice",
			},
			validators: []PrincipalValidator{
				func(p *ArmoryCloudPrincipal) error {
					return nil
				},
				func(p *ArmoryCloudPrincipal) error {
					return errors.New("failed validation")
				},
			},
			statusCode: http.StatusForbidden,
			errorMsg:   "failed validation",
		},
	}

	for _, c := range cases {
		test.Run(c.desc, func(t *testing.T) {
			a := &ArmoryCloudPrincipalService{
				JwtFetcher: c.fetcher,
			}
			if c.validators != nil {
				a.Validators = c.validators
			}
			h := a.ArmoryCloudPrincipalMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p, err := ExtractPrincipalFromContext(r.Context())
				assert.NoError(t, err, "Downstream should always have a principal in the request context")
				assert.NotNilf(t, p, "Downstream should always have a principal in the request context")
				if c.errorMsg != "" {
					assert.Equal(t, true, false, "Should never reach next handler in the chain")
				}
			}))
			r := httptest.NewRequest(http.MethodGet, "http://armory.io/", nil)
			for k, v := range c.headers {
				r.Header.Add(k, v)
			}
			recorder := httptest.NewRecorder()
			h.ServeHTTP(recorder, r)

			assert.Equal(t, c.statusCode, recorder.Code)
			var out struct {
				Message string `json:"error"`
			}
			if c.statusCode >= 400 {
				if err := json.NewDecoder(recorder.Result().Body).Decode(&out); err != nil {
					t.Fatal(err.Error())
				}
				assert.Equal(t, c.errorMsg, out.Message)
			}

			if c.assertion != nil {
				c.assertion(t, c)
			}
		})
	}
}
