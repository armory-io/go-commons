package server

import (
	"context"
	"encoding/json"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/logging"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type MyResponse struct {
	MyCoolResponse string
}

type ServerTestSuite struct {
	log *zap.SugaredLogger
	suite.Suite
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (s *ServerTestSuite) SetupSuite() {
	logger, _ := logging.StdArmoryDevLogger(zapcore.InfoLevel)
	s.log = logger.Sugar()
}

var noop = func(ctx context.Context, _ Void) (*Response[Void], serr.Error) { return nil, nil }

func (s *ServerTestSuite) TestGinHOF() {
	s.T().Run("ginHOF should populate the request context with expected server.RequestDetails", func(t *testing.T) {
		expected := &RequestDetails{
			Headers: map[string][]string{
				"Accept":       {"*/*"},
				"Content-Type": {"application/json"},
			},
			QueryParameters: map[string][]string{
				"some-multi-value-key":  {"value1", "value2"},
				"some-single-value-key": {"the-only-value"},
			},
			PathParameters: map[string]string{
				"foo": "bar",
			},
		}
		handler := &handlerDTO{
			AuthOptOut: true,
		}

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/id/bar?some-single-value-key=the-only-value&some-multi-value-key=value1&some-multi-value-key=value2")
		c.Params = gin.Params{
			gin.Param{
				Key:   "foo",
				Value: "bar",
			},
		}
		c.Request = &http.Request{
			Header: map[string][]string{
				"Accept":       {"*/*"},
				"Content-Type": {"application/json"},
			},
			Method: http.MethodGet,
			URL:    stubURL,
		}

		var actual *RequestDetails
		ginHOF(func(ctx context.Context, _ Void) (*Response[Void], serr.Error) {
			actual, _ = GetRequestDetailsFromContext(ctx)
			return nil, nil
		}, handler, nil, s.log)(c)
		assert.Equal(s.T(), expected, actual)
	})

	s.T().Run("ginHOF should return the expected API error if the principal isn't in the request context", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodGet,
			URL:    stubURL,
		}

		ginHOF(noop, &handlerDTO{}, nil, s.log)(c)
		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, invalidCredentialsError.Message, apiError.Errors[0].Message)
		assert.Equal(t, invalidCredentialsError.HttpStatusCode, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should the expected API error if the authN function fails", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodGet,
			URL:    stubURL,
		}

		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(c, &iam.ArmoryCloudPrincipal{
			Name: "s.archer@p4o.io",
		}))

		ginHOF(noop, &handlerDTO{
			AuthZValidators: []AuthZValidatorFn{
				func(p *iam.ArmoryCloudPrincipal) (string, bool) {
					return "the principal is invalid", false
				},
			},
		}, nil, s.log)(c)
		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, principalNotAuthorized.Message, apiError.Errors[0].Message)
		assert.Equal(t, principalNotAuthorized.HttpStatusCode, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should handle POST/PUT/PATCH requests that do not have a request or response body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		ginHOF(noop, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, s.log)(c)

		assert.Equal(t, http.StatusNoContent, recorder.Result().StatusCode)
		b, err := io.ReadAll(recorder.Result().Body)
		if err != nil {
			t.Fatalf(err.Error())
		}
		assert.Equal(t, 0, len(b))
	})

	s.T().Run("ginHOF should handle return the expected API Error if the request body is null", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		handlerFn := func(ctx context.Context, request struct {
			Name string `json:"name" validate:"required"`
		}) (*Response[Void], serr.Error) {
			return nil, nil
		}

		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errBodyRequired.Message, apiError.Errors[0].Message)
		assert.Equal(t, errBodyRequired.HttpStatusCode, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should handle return the expected API Error if the request body is invalid", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{}")),
		}

		handlerFn := func(ctx context.Context, request struct {
			Name string `json:"name" validate:"required"`
		}) (*Response[Void], serr.Error) {
			return nil, nil
		}

		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, "Key: 'Name' Error:Field validation for 'Name' failed on the 'required' tag", apiError.Errors[0].Message)
		assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should handle return the expected API Error if the request body is malformed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{notvalidjson")),
		}

		handlerFn := func(ctx context.Context, request struct {
			Name string `json:"name" validate:"required"`
		}) (*Response[Void], serr.Error) {
			return nil, nil
		}

		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errFailedToUnmarshalRequest.Message, apiError.Errors[0].Message)
		assert.Equal(t, errFailedToUnmarshalRequest.HttpStatusCode, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should handle return the expected API Error if the request method isn't handleable", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}},
			Method: http.MethodConnect,
			URL:    stubURL,
		}

		ginHOF(noop, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errMethodNotAllowed.Message, apiError.Errors[0].Message)
		assert.Equal(t, errMethodNotAllowed.HttpStatusCode, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should return the expected response when a handler returns an API Error", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		handlerFn := func(ctx context.Context, _ Void) (*Response[Void], serr.Error) {
			return nil, serr.NewSimpleErrorWithStatusCode("foo", 404, nil)
		}
		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, "foo", apiError.Errors[0].Message)
		assert.Equal(t, 404, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should return the expected response when a handler returns a valid response", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		type MyResponse struct {
			MyCoolResponse string
		}

		handlerFn := func(ctx context.Context, _ Void) (*Response[MyResponse], serr.Error) {
			return SimpleResponse(MyResponse{MyCoolResponse: "Hey"}), nil
		}
		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusCreated,
			AuthOptOut: true,
		}, nil, s.log)(c)

		res := ExtractResponse[MyResponse](t, recorder)
		assert.Equal(t, "Hey", res.MyCoolResponse)
		assert.Equal(t, http.StatusCreated, recorder.Result().StatusCode)
	})

	s.T().Run("ginHOF should return the expected response when a handler returns a valid response and overrides the status code and adds custom headers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		handlerFn := func(ctx context.Context, _ Void) (*Response[MyResponse], serr.Error) {
			return &Response[MyResponse]{
				Body:       MyResponse{MyCoolResponse: "Let's go to the tea party"},
				StatusCode: http.StatusTeapot,
				Headers: map[string][]string{
					"Location": {"Wonderland"},
				},
			}, nil
		}
		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusOK,
			AuthOptOut: true,
		}, nil, s.log)(c)

		res := ExtractResponse[MyResponse](t, recorder)
		assert.Equal(t, "Let's go to the tea party", res.MyCoolResponse)
		assert.Equal(t, http.StatusTeapot, recorder.Result().StatusCode)
		assert.Equal(t, "Wonderland", recorder.Header().Get("Location"))
	})

	s.T().Run("ginHOF should return the expected api error when a handler returns a response that has a nil body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
		}

		handlerFn := func(ctx context.Context, _ Void) (*Response[*MyResponse], serr.Error) {
			return &Response[*MyResponse]{
				Body: nil,
			}, nil
		}

		ginHOF(handlerFn, &handlerDTO{
			StatusCode: http.StatusOK,
			AuthOptOut: true,
		}, nil, s.log)(c)

		assert.Equal(t, errServerFailedToProduceExpectedResponse.HttpStatusCode, recorder.Result().StatusCode)
		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errServerFailedToProduceExpectedResponse.Message, apiError.Errors[0].Message)
	})
}

func ExtractApiError(t *testing.T, recorder *httptest.ResponseRecorder) *serr.ResponseContract {
	error := &serr.ResponseContract{}
	bytes, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("Failed to read body")
	}
	err = json.Unmarshal(bytes, &error)
	if err != nil {
		t.Fatalf("Failed unmarshel response body to serr.ResponseContract")
	}
	return error
}

func ExtractResponse[RESPONSE any](t *testing.T, recorder *httptest.ResponseRecorder) RESPONSE {
	var response RESPONSE
	bytes, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("Failed to read body")
	}
	err = json.Unmarshal(bytes, &response)
	if err != nil {
		t.Fatalf("Failed unmarshel response body to serr.ResponseContract")
	}
	return response
}
