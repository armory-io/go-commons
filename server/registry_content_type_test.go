package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/armory-io/go-commons/awaitility"
	armoryhttp "github.com/armory-io/go-commons/http"
	"github.com/armory-io/go-commons/logging"
	"github.com/armory-io/go-commons/management/info"
	"github.com/armory-io/go-commons/metadata"
	metrics2 "github.com/armory-io/go-commons/metrics"
	"github.com/armory-io/go-commons/server/serr"
	"github.com/go-playground/validator/v10"
	"github.com/golang/mock/gomock"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally/v4"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

type (
	ContentTypesTestSuite struct {
		log *zap.SugaredLogger
		suite.Suite
		controller Controller
		lc         *fxtest.Lifecycle
		client     *http.Client
		baseUrl    string
	}

	contentTypeController struct {
	}

	testTimer struct {
	}
)

func (d *contentTypeController) Handlers() []Handler {
	return []Handler{
		NewHandler(d.readyHandler, HandlerConfig{
			Path:       "/ready",
			Method:     http.MethodGet,
			Produces:   "text/plain",
			AuthOptOut: true,
		}),
		NewHandler(d.defaultHandler, HandlerConfig{
			Default:    true,
			AuthOptOut: true,
			Method:     http.MethodPost,
			Consumes:   "*/*",
			Path:       "/api",
		}),
		NewHandler(d.acceptsJsonProducesYaml, HandlerConfig{
			AuthOptOut: true,
			Method:     http.MethodPost,
			Produces:   "application/yaml",
			Path:       "/api",
		}),
		NewHandler(d.dedicatedJsonEndpoint, HandlerConfig{
			AuthOptOut: true,
			Method:     http.MethodPost,
			Consumes:   applicationJSON,
			Produces:   applicationJSON,
			Path:       "/api",
		}),
		NewHandler(d.acceptsYamlProducesYaml, HandlerConfig{
			AuthOptOut: true,
			Method:     http.MethodPost,
			Consumes:   "application/yaml",
			Produces:   "application/yaml",
			Path:       "/api",
		}),
	}
}

func (*contentTypeController) defaultHandler(_ context.Context, _ Void) (*Response[map[string]string], serr.Error) {
	result := make(map[string]string)
	result["result"] = "defaultHandler"
	return SimpleResponse[map[string]string](result), nil
}

func (*contentTypeController) acceptsJsonProducesYaml(_ context.Context, _ Void) (*Response[string], serr.Error) {
	result :=
		`
  result: "yamlHandler"      
`
	return SimpleResponse[string](result), nil
}

func (*contentTypeController) acceptsYamlProducesYaml(_ context.Context, bs []byte) (*Response[string], serr.Error) {
	var result map[string]string
	_ = yaml.Unmarshal(bs, &result)
	result["result"] = result["in"] + "DedicatedYaml"
	resultBytes, _ := yaml.Marshal(result)
	return SimpleResponse[string](string(resultBytes)), nil
}

func (*contentTypeController) dedicatedJsonEndpoint(_ context.Context, req map[string]string) (*Response[map[string]string], serr.Error) {
	req["result"] = req["in"] + "DedicatedJson"
	return SimpleResponse[map[string]string](req), nil
}

func (s *ContentTypesTestSuite) TestContentTypes() {
	cases := []struct {
		name              string
		requestBody       []byte
		contentTypeHeader string
		acceptHeaders     []string
		handler           func(t *testing.T, response *http.Response)
	}{
		{
			name: "default handler is executed when no Content-Type or Accept headers are specified",
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, json.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "defaultHandler", result["result"])
			},
		},
		{
			name:          "default handler is executed when no Content-Type header is specified and none Accept matches",
			acceptHeaders: []string{"application/foo.1", "application/foo.2", "application/foo.3"},
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, json.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "defaultHandler", result["result"])
			},
		},
		{
			name:          "handler with matching Accept header is executed when no Content-Type is specified",
			acceptHeaders: []string{"application/yaml"},
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, yaml.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "yamlHandler", result["result"])
			},
		},
		{
			name:              "handler with exact match is selected",
			acceptHeaders:     []string{applicationJSON},
			contentTypeHeader: applicationJSON,
			requestBody:       lo.Must(json.Marshal(map[string]string{"in": "requested"})),
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, json.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "requestedDedicatedJson", result["result"])
			},
		},
		{
			name:              "handler with exact match is selected with content type charset",
			acceptHeaders:     []string{applicationJSON},
			contentTypeHeader: applicationJSON + ";charset=utf-8",
			requestBody:       lo.Must(json.Marshal(map[string]string{"in": "requested"})),
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, json.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "requestedDedicatedJson", result["result"])
			},
		},
		{
			name:          "handler without content type but with multiple accepts selects matching accept",
			acceptHeaders: []string{"text/html", "text/plain", "application/yaml", "*/*"},
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, yaml.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "yamlHandler", result["result"])
			},
		},
		{
			name:              "handler with content type and multiple accepts selects matching accept",
			contentTypeHeader: "application/yaml;charset=utf-8",
			acceptHeaders:     []string{"text/html", "text/plain", "application/yaml", "*/*"},
			requestBody:       lo.Must(yaml.Marshal(map[string]string{"in": "requested"})),
			handler: func(t *testing.T, response *http.Response) {
				assert.Equal(t, http.StatusOK, response.StatusCode)
				var result map[string]string
				assert.NoError(t, yaml.NewDecoder(response.Body).Decode(&result))
				assert.Equal(t, "requestedDedicatedYaml", result["result"])
			},
		},
	}

	for _, testCase := range cases {
		s.T().Run(testCase.name, func(t *testing.T) {
			reader := lo.
				IfF(testCase.requestBody != nil, func() io.Reader {
					return bytes.NewReader(testCase.requestBody)
				}).
				Else(nil)

			request, err := http.NewRequest(http.MethodPost, s.baseUrl+"api", reader)
			if testCase.contentTypeHeader != "" {
				request.Header.Add("Content-Type", testCase.contentTypeHeader)
			}
			for _, accepts := range testCase.acceptHeaders {
				request.Header.Add("Accept", accepts)
			}
			assert.NoError(t, err)
			if err != nil {
				response, err := s.client.Do(request)

				assert.NoError(t, err)
				if err == nil && response != nil {
					defer response.Body.Close()
					testCase.handler(t, response)
				}
			}
		})
	}
}

func TestContentTypesTestSuite(t *testing.T) {
	suite.Run(t, new(ContentTypesTestSuite))
}

func (s *ContentTypesTestSuite) SetupSuite() {
	logger, _ := logging.StdArmoryDevLogger(zapcore.InfoLevel)
	s.log = logger.Sugar()
	s.controller = newController()
	port, err := getFreePort()
	if err != nil {
		s.T().Fail()
		return
	}

	s.lc = fxtest.NewLifecycle(s.T())
	config := armoryhttp.HTTP{
		Prefix: "",
		Host:   "127.0.0.1",
		Port:   port,
	}
	s.client = &http.Client{}
	s.baseUrl = fmt.Sprintf("http://localhost:%d/", port)
	metrics := metrics2.NewMockMetricsSvc(gomock.NewController(s.T()))
	metrics.EXPECT().TimerWithTags(gomock.Any(), gomock.Any()).Return(&testTimer{})

	is := &info.InfoService{}

	err = configureServer("http",
		s.lc,
		config,
		RequestLoggingConfiguration{Enabled: false},
		SPAConfiguration{Enabled: false},
		nil,
		s.log,
		metrics,
		metadata.ApplicationMetadata{},
		is,
		false,
		validator.New(),
		s.controller.Controller)
	if err != nil {
		s.T().Fail()
		return
	}
	err = s.lc.Start(context.Background())
	if err != nil {
		s.T().Fail()
		return
	}
	err = awaitility.Await(time.Second, time.Second*10, func() bool {
		req, _ := http.NewRequest(http.MethodGet, s.baseUrl+"ready", nil)
		resp, err := s.client.Do(req)
		return err == nil && resp != nil && resp.StatusCode == http.StatusOK
	})
}

func (s *ContentTypesTestSuite) TearDownSuite() {
	_ = s.lc.Stop(context.Background())
}

func (*contentTypeController) readyHandler(_ context.Context, _ Void) (*Response[string], serr.Error) {
	return SimpleResponse[string]("ready"), nil
}

func newController() Controller {
	return Controller{
		Controller: &contentTypeController{},
	}
}

func getFreePort() (uint32, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return uint32(l.Addr().(*net.TCPAddr).Port), nil
}

func (testTimer) Record(_ time.Duration) {
}

func (testTimer) Start() tally.Stopwatch {
	return tally.Stopwatch{}
}
