package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/logging"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/samber/lo"
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
			RequestPath: "/id/bar",
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
			actual, _ = ExtractRequestDetailsFromContext(ctx)
			return nil, nil
		}, nil, handler, nil, &HandlerExtensionPoints{}, s.log)(c)
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

		ginHOF(noop, nil, &handlerDTO{}, nil, &HandlerExtensionPoints{}, s.log)(c)
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

		ctx := c.Request.Context()
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(ctx, &iam.ArmoryCloudPrincipal{
			Name: "s.archer@p4o.io",
		}))

		ginHOF(noop, nil, &handlerDTO{
			AuthZValidators: []AuthZValidatorV2Fn{
				func(_ context.Context, p *iam.ArmoryCloudPrincipal) (string, bool) {
					return "the principal is invalid", false
				},
			},
		}, nil, &HandlerExtensionPoints{}, s.log)(c)
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

		ginHOF(noop, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

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

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

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

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), &HandlerExtensionPoints{}, s.log)(c)

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
			Body:   io.NopCloser(strings.NewReader("{ \"foo\":\"bar\",\n \n\"invalidJson\":\"not closed")),
		}

		handlerFn := func(ctx context.Context, request struct {
			Name string `json:"name" validate:"required"`
		}) (*Response[Void], serr.Error) {
			return nil, nil
		}

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), &HandlerExtensionPoints{}, s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errFailedToUnmarshalRequest.Message, apiError.Errors[0].Message)
		assert.Equal(t, errFailedToUnmarshalRequest.HttpStatusCode, recorder.Result().StatusCode)
		metadata := apiError.Errors[0].Metadata
		assert.Equal(t, "unexpected end of JSON input", metadata["reason"])
		assert.Equal(t, float64(25), metadata["column"])
		assert.Equal(t, float64(2), metadata["line"])
		assert.Equal(t, float64(42), metadata["offset"])
	})

	s.T().Run("ginHOF should handle return the expected API Error if the request body doesn't match the desired data types", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{\n\"body\": {\n \"value\": \"ten\"}\n}")),
		}

		handlerFn := func(ctx context.Context, request struct {
			Body struct {
				IntegerValue int `json:"value" validate:"required"`
			}
		}) (*Response[Void], serr.Error) {
			return nil, nil
		}

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), &HandlerExtensionPoints{}, s.log)(c)

		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, "Failed to unmarshal request", apiError.Errors[0].Message)
		assert.Equal(t, http.StatusBadRequest, recorder.Result().StatusCode)
		metadata := apiError.Errors[0].Metadata
		assert.Equal(t, "cannot unmarshal data", metadata["reason"])
		assert.Equal(t, float64(15), metadata["column"])
		assert.Equal(t, float64(2), metadata["line"])
		assert.Equal(t, float64(27), metadata["offset"])
		assert.Equal(t, "int", metadata["expectedType"])
		assert.Equal(t, "string", metadata["providedType"])
		assert.Equal(t, "Body.value", metadata["path"])
	})

	s.T().Run("ginHOF should fill in request body struct defaults", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{"Accept": {"application/json"}, "Content-Type": {"application/json"}},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{}")),
		}

		var actual string
		handlerFn := func(ctx context.Context, request struct {
			Name string `json:"name" default:"fill-me-in"`
		}) (*Response[Void], serr.Error) {
			actual = request.Name
			return nil, nil
		}

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), &HandlerExtensionPoints{}, s.log)(c)

		s.Equal("fill-me-in", actual)
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

		ginHOF(noop, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, validator.New(), &HandlerExtensionPoints{}, s.log)(c)

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
		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusNoContent,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

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
		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusCreated,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

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
		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusOK,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

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

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusOK,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

		assert.Equal(t, errServerFailedToProduceExpectedResponse.HttpStatusCode, recorder.Result().StatusCode)
		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errServerFailedToProduceExpectedResponse.Message, apiError.Errors[0].Message)
	})

	s.T().Run("ginHOF should recover from a panic and returned a well-formed error", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/some-endpoint")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}

		handlerFn := func(ctx context.Context, _ Void) (*Response[string], serr.Error) {
			unsafeStruct := &Widget{}
			name := unsafeStruct.subWidget.name // <- Should cause NPE Panic
			return SimpleResponse(name), nil
		}

		ginHOF(handlerFn, nil, &handlerDTO{
			StatusCode: http.StatusOK,
			AuthOptOut: true,
		}, nil, &HandlerExtensionPoints{}, s.log)(c)

		assert.Equal(t, errInternalServerError.HttpStatusCode, recorder.Result().StatusCode)
		apiError := ExtractApiError(t, recorder)
		assert.Equal(t, errInternalServerError.Message, apiError.Errors[0].Message)
	})

	s.T().Run("parametrized handler will get context parameters from path", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		c.Params = gin.Params{
			gin.Param{
				Key:   "key1",
				Value: "hello world",
			},
			gin.Param{
				Key:   "key2",
				Value: "1234",
			},
		}

		handler := New1ArgHandler(func(ctx context.Context, request Void, arg1 PathParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "hello world", arg1.Key1)
			assert.Equal(t, 1234, arg1.Key2)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api/key1/:key1/key2/:key2",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("parametrized handler will get context parameters from query", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com?keyA=world&keyB=4321")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		handler := New1ArgHandler(func(ctx context.Context, request Void, arg1 QueryParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "world", arg1.KeyA[0])
			assert.Equal(t, 4321, arg1.KeyB[0])
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api?keyA=world&keyB=4321",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("parametrized handler will get context parameters from headers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")
		c.Request = &http.Request{
			Header: map[string][]string{
				"x-org-id": []string{"header-value"},
			},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		handler := New1ArgHandler(func(ctx context.Context, request Void, arg1 HeaderParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "header-value", arg1.OrgIdParameter[0])
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("parametrized handler will get armory principal as argument", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		ctx := c.Request.Context()
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(ctx, &iam.ArmoryCloudPrincipal{
			Name: "happy@user.io",
		}))
		handler := New1ArgHandler(func(ctx context.Context, request Void, arg1 ArmoryPrincipalArgument) (*Response[string], serr.Error) {
			assert.Equal(t, "happy@user.io", arg1.Name)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:   "",
			Method: http.MethodGet,
			AuthZValidator: func(p *iam.ArmoryCloudPrincipal) (string, bool) {
				return "", true
			},
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: false,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("parametrized handler with 2 args works", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com?keyA=world&keyB=4321")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		ctx := c.Request.Context()
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(ctx, &iam.ArmoryCloudPrincipal{
			Name: "happy@user.io",
		}))
		handler := New2ArgHandler(func(ctx context.Context, request Void, arg1 ArmoryPrincipalArgument, arg2 QueryParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "happy@user.io", arg1.Name)
			assert.Equal(t, arg2.KeyA[0], "world")
			assert.Equal(t, arg2.KeyB[0], 4321)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:   "",
			Method: http.MethodGet,
			AuthZValidator: func(p *iam.ArmoryCloudPrincipal) (string, bool) {
				return "", true
			},
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: false,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("parametrized handler with 3 args works", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com?keyA=world&keyB=4321")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodGet,
			URL:    stubURL,
		}
		ctx := c.Request.Context()
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(ctx, &iam.ArmoryCloudPrincipal{
			Name: "happy@user.io",
		}))
		c.Params = gin.Params{
			gin.Param{
				Key:   "key1",
				Value: "hello world",
			},
			gin.Param{
				Key:   "key2",
				Value: "1234",
			},
		}
		handler := New3ArgHandler(func(ctx context.Context, request Void, arg1 ArmoryPrincipalArgument, arg2 QueryParameters, arg3 PathParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "happy@user.io", arg1.Name)
			assert.Equal(t, arg2.KeyA[0], "world")
			assert.Equal(t, arg2.KeyB[0], 4321)
			assert.Equal(t, arg3.Key1, "hello world")
			assert.Equal(t, arg3.Key2, 1234)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:   "",
			Method: http.MethodGet,
			AuthZValidator: func(p *iam.ArmoryCloudPrincipal) (string, bool) {
				return "", true
			},
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: false,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler with no extra params will trigger 'beforeValidation' callback and populate request body with data from path parameters", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{ \"Value\": \"body-content\"}")),
		}

		handler := NewHandler(func(ctx context.Context, request TestRequestBody) (*Response[string], serr.Error) {
			assert.Equal(t, "BODY-CONTENT", request.Value)
			assert.Equal(t, "1234567890", request.Key1)
			assert.Equal(t, 1, *request.Key2)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api",
			Method:         http.MethodPost,
			AuthZValidator: nil,
		}).RegisterBeforeValidationHandler(func(body *TestRequestBody) {
			body.Value = strings.ToUpper(body.Value)
			body.Key1 = "1234567890"
			body.Key2 = lo.ToPtr(1)
		})

		handlerFn := handler.GetGinHandlerFn(s.log, validator.New(), &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler with 1 extra param will trigger 'beforeValidation' callback and populate request body with data from path parameters", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{ \"Value\": \"body-content\"}")),
		}
		c.Params = gin.Params{
			gin.Param{
				Key:   "key1",
				Value: "-must-be-provided-",
			},
			gin.Param{
				Key:   "key2",
				Value: "1234",
			},
		}

		handler := New1ArgHandler(func(ctx context.Context, request TestRequestBody, arg1 PathParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "body-content", request.Value)
			assert.Equal(t, 1234, *request.Key2)
			assert.Equal(t, "-must-be-provided-", request.Key1)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api/key1/:key1/key2/:key2",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		}).RegisterBeforeValidationHandler(func(body *TestRequestBody, arg1 *PathParameters) {
			body.Key1 = arg1.Key1
			body.Key2 = lo.ToPtr(arg1.Key2)
		})

		handlerFn := handler.GetGinHandlerFn(s.log, validator.New(), &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler with 2 extra params will trigger 'beforeValidation' callback and populate request body with data from path parameters", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/?keyA=!hello world!&keyB=2")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{ \"Value\": \"body-content\"}")),
		}
		c.Params = gin.Params{
			gin.Param{
				Key:   "key1",
				Value: "-must-be-provided-",
			},
			gin.Param{
				Key:   "key2",
				Value: "1234",
			},
		}

		handler := New2ArgHandler(func(ctx context.Context, request TestRequestBody, arg1 PathParameters, arg2 QueryParameters) (*Response[string], serr.Error) {
			assert.Equal(t, "body-content", request.Value)
			assert.Equal(t, 1234*2, *request.Key2)
			assert.Equal(t, "-must-be-provided-!hello world!", request.Key1)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api/key1/:key1/key2/:key2",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		}).RegisterBeforeValidationHandler(func(body *TestRequestBody, arg1 *PathParameters, arg2 *QueryParameters) {
			body.Key1 = arg1.Key1 + arg2.KeyA[0]
			body.Key2 = lo.ToPtr(arg1.Key2 * arg2.KeyB[0])
		})

		handlerFn := handler.GetGinHandlerFn(s.log, validator.New(), &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler with 3 extra params will trigger 'beforeValidation' callback and populate request body with data from path parameters", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com/?keyA=!hello world!&keyB=2")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{ \"Value\": \"body-content\"}")),
		}
		c.Params = gin.Params{
			gin.Param{
				Key:   "key1",
				Value: "-must-be-provided-",
			},
			gin.Param{
				Key:   "key2",
				Value: "1234",
			},
		}
		c.Request = c.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(c.Request.Context(), &iam.ArmoryCloudPrincipal{
			Name: "test user",
		}))

		handler := New3ArgHandler(func(ctx context.Context, request TestRequestBody, arg1 PathParameters, arg2 QueryParameters, arg3 ArmoryPrincipalArgument) (*Response[string], serr.Error) {
			assert.Equal(t, "body-content", request.Value)
			assert.Equal(t, 1234*2, *request.Key2)
			assert.Equal(t, "-must-be-provided-!hello world!test user", request.Key1)
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "/api/key1/:key1/key2/:key2",
			Method:         http.MethodGet,
			AuthZValidator: nil,
		}).RegisterBeforeValidationHandler(func(body *TestRequestBody, arg1 *PathParameters, arg2 *QueryParameters, arg3 *ArmoryPrincipalArgument) {
			body.Key1 = arg1.Key1 + arg2.KeyA[0] + arg3.Name
			body.Key2 = lo.ToPtr(arg1.Key2 * arg2.KeyB[0])
		})

		handlerFn := handler.GetGinHandlerFn(s.log, validator.New(), &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler will work with []string body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("[ \"arg1\", \"arg2\"]")),
		}
		handler := NewHandler(func(ctx context.Context, request []string) (*Response[string], serr.Error) {
			assert.Equal(t, "arg1", request[0])
			assert.Equal(t, "arg2", request[1])
			return SimpleResponse("ok"), nil

		}, HandlerConfig{
			Path:           "",
			Method:         http.MethodPost,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
	})

	s.T().Run("handler will work with []byte request / response parameters", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")

		var buffer bytes.Buffer
		err := binary.Write(&buffer, binary.BigEndian, []byte("hello world!"))
		if err != nil {
			t.Fatal("failed to marshal body", err)
		}
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(bytes.NewReader(buffer.Bytes())),
		}
		handler := NewHandler(func(ctx context.Context, body []byte) (*Response[[]byte], serr.Error) {
			result := reverse(body)
			return SimpleResponse(result), nil
		}, HandlerConfig{
			Path:           "",
			Method:         http.MethodPost,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
			Produces:   "text/plain",
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
		assert.Equal(t, "!dlrow olleh", string(recorder.Body.Bytes()))
	})

	s.T().Run("handler will work with response processor for text based handlers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")

		var buffer bytes.Buffer
		err := binary.Write(&buffer, binary.BigEndian, []byte("hello world!"))
		if err != nil {
			t.Fatal("failed to marshal body", err)
		}
		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(bytes.NewReader(buffer.Bytes())),
		}
		handler := NewHandler(func(ctx context.Context, body []byte) (*Response[[]byte], serr.Error) {
			result := reverse(body)
			return SimpleResponse(result), nil
		}, HandlerConfig{
			Path:           "",
			Method:         http.MethodPost,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, nil, &handlerDTO{
			AuthOptOut: true,
			Produces:   "text/plain",
			ResponseProcessors: []ResponseProcessorFn{func(_ context.Context, body []byte) ([]byte, serr.Error) {
				return reverse(body), nil
			}},
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
		assert.Equal(t, "hello world!", string(recorder.Body.Bytes()))
	})

	s.T().Run("handler will work with response processor for JSON based handlers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		stubURL, _ := url.ParseRequestURI("https://example.com")

		c.Request = &http.Request{
			Header: map[string][]string{},
			Method: http.MethodPost,
			URL:    stubURL,
			Body:   io.NopCloser(strings.NewReader("{\"title\": \"Brave new world\"}")),
		}
		handler := NewHandler(func(ctx context.Context, body Book) (*Response[Book], serr.Error) {
			body.Author = "Aldous Huxley"
			return SimpleResponse(body), nil
		}, HandlerConfig{
			Path:           "",
			Method:         http.MethodPost,
			AuthZValidator: nil,
		})

		handlerFn := handler.GetGinHandlerFn(s.log, validator.New(), &handlerDTO{
			AuthOptOut: true,
			Produces:   "application/json",
			ResponseProcessors: []ResponseProcessorFn{func(_ context.Context, body []byte) ([]byte, serr.Error) {
				var book Book
				_ = json.Unmarshal(body, &book)
				book.Author = strings.ToUpper(book.Author)
				book.Title = strings.ToUpper(book.Title)
				body, _ = json.Marshal(book)
				return body, nil
			}},
		})
		handlerFn(c)
		assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)
		var book Book
		_ = json.Unmarshal(recorder.Body.Bytes(), &book)
		assert.Equal(t, "BRAVE NEW WORLD", book.Title)
		assert.Equal(t, "ALDOUS HUXLEY", book.Author)
	})

	s.T().Run("handler test utils make like a bit easier - part 1", func(t *testing.T) {
		htc := NewHandlerTestContext(t, newDummyController().Controller, HandlerByLabel("simple"))
		ctx, handler, resp := htc.
			WithBody(t, TestRequestBody{}).
			WithValidator(t, validator.New()).
			WithHttpMethod(t, http.MethodPost).
			WithPathParameters(t, "key1", "from path").
			WithRequestHeaders(t, "x-org-id", "from header").
			WithRequestUrl(t, "https://foo.bar?keyA=from query").
			BuildHandler(t)

		handler(ctx)

		result, code := ExtractResponseDataAndCode[string](t, resp)

		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "from query,from header,from path", *result)
	})

	s.T().Run("handler test utils make like a bit easier - part 2", func(t *testing.T) {
		htc := NewHandlerTestContext(t, newDummyController().Controller, HandlerByLabel("passThrough"))
		ctx, handler, resp := htc.
			WithRawBody(t, []byte("hello")).
			BuildHandler(t)

		handler(ctx)

		result, code := ExtractResponseDataAndCode[string](t, resp)

		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "--hello--", *result)
	})

	s.T().Run("handler test utils make like a bit easier - part 3", func(t *testing.T) {
		htc := NewHandlerTestContext(t, newDummyController().Controller, HandlerByLabel("passThroughWithPrincipal"))
		ctx, handler, resp := htc.
			WithJSONBody(t, "{\"prompt\": \"Welcome mister \"}").
			WithPrincipal(t, &iam.ArmoryCloudPrincipal{Name: "Bond"}).
			BuildHandler(t)

		handler(ctx)

		result, code := ExtractResponseDataAndCode[string](t, resp)

		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "Welcome mister Bond", *result)
	})
}

type Book struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

func reverse(input []byte) []byte {
	if len(input) == 0 {
		return input
	}
	return append(reverse(input[1:]), input[0])
}

type PathParameters struct {
	Key1 string
	Key2 int
}

func (PathParameters) Source() ArgumentDataSource {
	return PathContextSource
}

type QueryParameters struct {
	KeyA []string
	KeyB []int
}

func (QueryParameters) Source() ArgumentDataSource {
	return QueryContextSource
}

type HeaderParameters struct {
	OrgIdParameter []string `mapstructure:"x-org-id"`
}

func (HeaderParameters) Source() ArgumentDataSource {
	return HeaderContextSource
}

type Widget struct {
	subWidget *SubWidget
}

type SubWidget struct {
	name string
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

type TestRequestBody struct {
	Value string `validate:"required"`
	Key1  string `validate:"required,min=10"`
	Key2  *int   `validate:"required"`
}

type dummyController struct {
}

func (*dummyController) SimpleOperation(c context.Context, body TestRequestBody, arg1 QueryParameters, arg2 HeaderParameters, arg3 PathParameters) (*Response[string], serr.Error) {
	return SimpleResponse(body.Value), nil
}
func (*dummyController) StringPassThrough(c context.Context, body []byte) (*Response[string], serr.Error) {
	return SimpleResponse("--" + string(body) + "--"), nil
}

func (*dummyController) StringPassThroughWithPrincipal(c context.Context, body struct{ Prompt string }, p ArmoryPrincipalArgument) (*Response[string], serr.Error) {
	return SimpleResponse(body.Prompt + p.Name), nil
}

func (d *dummyController) Handlers() []Handler {
	return []Handler{
		New3ArgHandler(d.SimpleOperation, HandlerConfig{
			Path:       "/foo/bar",
			Method:     http.MethodPost,
			AuthOptOut: true,
			Label:      "simple",
		}).RegisterBeforeValidationHandler(func(body *TestRequestBody, a1 *QueryParameters, a2 *HeaderParameters, a3 *PathParameters) {
			body.Value = strings.Join([]string{a1.KeyA[0], a2.OrgIdParameter[0], a3.Key1}, ",")
			body.Key1 = "filled in to pass the body validation later"
			body.Key2 = lo.ToPtr(123)
		}),

		NewHandler(d.StringPassThrough, HandlerConfig{
			Path:       "/foo/bar",
			Method:     http.MethodPost,
			AuthOptOut: true,
			Label:      "passThrough",
		}),

		New1ArgHandler(d.StringPassThroughWithPrincipal, HandlerConfig{
			Path:       "/foo/bar",
			Method:     http.MethodPost,
			AuthOptOut: true,
			Label:      "passThroughWithPrincipal",
		}),
	}
}

func newDummyController() Controller {
	return Controller{
		Controller: &dummyController{},
	}
}
