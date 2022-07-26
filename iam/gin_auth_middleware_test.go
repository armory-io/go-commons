package iam

import (
	"encoding/json"
	armoryhttp "github.com/armory-io/lib-go-armory-cloud-commons/http"
	"github.com/gin-gonic/gin"
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

func TestGinPrincipalMiddleware(test *testing.T) {
	type PrincipalServiceTest struct {
		desc       string
		fetcher    *MockJwtFetcher
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
	}

	for _, c := range cases {
		test.Run(c.desc, func(t *testing.T) {
			a := &ArmoryCloudPrincipalService{
				JwtFetcher: c.fetcher,
			}

			g := gin.Default()

			g.Use(GinAuthMiddleware(a))
			g.Use(func(gc *gin.Context) {
				p, err := ExtractPrincipalFromContext(gc.Request.Context())
				assert.NoError(t, err, "Downstream should always have a principal in the request context")
				assert.NotNilf(t, p, "Downstream should always have a principal in the request context")
				if c.errorMsg != "" {
					assert.Equal(t, true, false, "Should never reach next handler in the chain")
				}
			})

			g.GET("/", func(gc *gin.Context) {
				gc.JSON(200, map[string]string{
					"hello": "from the other side",
				})
			})

			s := httptest.NewServer(g)

			r, err := http.NewRequest(http.MethodGet, s.URL, nil)
			assert.NoError(t, err)

			for k, v := range c.headers {
				r.Header.Add(k, v)
			}

			response, err := http.DefaultClient.Do(r)
			assert.NoError(t, err)

			assert.Equal(t, c.statusCode, response.StatusCode)
			if c.statusCode >= 400 {
				defer func() { assert.NoError(t, response.Body.Close()) }()

				var out armoryhttp.BackstopError
				if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
					t.Fatal(err.Error())
				}

				assert.Equal(t, c.errorMsg, out.Errors[0].Message)
			}

			if c.assertion != nil {
				c.assertion(t, c)
			}
		})
	}
}
