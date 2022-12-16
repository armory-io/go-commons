package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/armory-io/go-commons/iam"
	"github.com/armory-io/go-commons/logging"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type (
	HandlerTestContext struct {
		validate        *validator.Validate
		controller      IController
		selectedHandler Handler
		recorder        *httptest.ResponseRecorder
		ginContext      *gin.Context
		stubUrl         *url.URL
		method          string
		headers         http.Header
		params          gin.Params
		principal       *iam.ArmoryCloudPrincipal
		logger          *zap.SugaredLogger
		body            io.ReadCloser
	}

	HandlerSelector func(handler Handler) bool
)

var (
	HandlerByLabel = func(label string) HandlerSelector {
		return func(handler Handler) bool {
			return handler.Config().Label == label
		}
	}
)

func NewHandlerTestContext(t *testing.T, target IController, selector HandlerSelector) *HandlerTestContext {
	found := 0

	u, _ := url.ParseRequestURI("http://localhost/")
	logger, _ := logging.StdArmoryDevLogger(zapcore.InfoLevel)

	result := HandlerTestContext{
		stubUrl:    u,
		headers:    http.Header{},
		validate:   validator.New(),
		logger:     logger.Sugar(),
		controller: target,
	}

	for _, h := range target.Handlers() {
		if selector(h) {
			found++
			result.selectedHandler = h
			result.method = h.Config().Method
		}
	}

	if found != 1 {
		t.Fatal(fmt.Sprintf("found %d handlers, but expected exactly 1", found))
	}

	result.recorder = httptest.NewRecorder()
	c, _ := gin.CreateTestContext(result.recorder)
	result.ginContext = c

	return &result
}

func (h *HandlerTestContext) WithRequestUrl(t *testing.T, stubUrl string) *HandlerTestContext {
	var err error
	h.stubUrl, err = url.ParseRequestURI(stubUrl)
	if err != nil {
		t.Fatal("failed to parse url", err)
	}
	return h
}

func (h *HandlerTestContext) WithValidator(_ *testing.T, v *validator.Validate) *HandlerTestContext {
	h.validate = v
	return h
}

func (h *HandlerTestContext) WithHttpMethod(_ *testing.T, method string) *HandlerTestContext {
	h.method = method
	return h
}

func (h *HandlerTestContext) WithRequestHeaders(t *testing.T, pairs ...string) *HandlerTestContext {
	if len(pairs)%2 != 0 {
		t.Fatal("expected collection of {key, value} pairs")
	}
	for i := 0; i < len(pairs); i += 2 {
		h.headers.Add(pairs[i], pairs[i+1])
	}
	return h
}

func (h *HandlerTestContext) WithPathParameters(t *testing.T, pairs ...string) *HandlerTestContext {
	if len(pairs)%2 != 0 {
		t.Fatal("expected collection of {key, value} pairs")
	}
	for i := 0; i < len(pairs); i += 2 {
		h.params = append(h.params, gin.Param{Key: pairs[i], Value: pairs[i+1]})
	}

	return h
}

func (h *HandlerTestContext) WithPrincipal(_ *testing.T, principal *iam.ArmoryCloudPrincipal) *HandlerTestContext {
	h.principal = principal
	return h
}

func (h *HandlerTestContext) WithLogger(_ *testing.T, logger *zap.SugaredLogger) *HandlerTestContext {
	h.logger = logger
	return h
}

func (h *HandlerTestContext) WithJSONBody(_ *testing.T, body string) *HandlerTestContext {
	h.body = io.NopCloser(strings.NewReader(body))
	return h
}

func (h *HandlerTestContext) WithBody(t *testing.T, body interface{}) *HandlerTestContext {
	bytes, err := json.Marshal(body)
	if err != nil {
		t.Fatal("failed to marshal body", err)
	}
	h.body = io.NopCloser(strings.NewReader(string(bytes)))
	return h
}

func (h *HandlerTestContext) WithRawBody(t *testing.T, body any) *HandlerTestContext {
	var buffer bytes.Buffer
	err := binary.Write(&buffer, binary.BigEndian, body)
	if err != nil {
		t.Fatal("failed to marshal body", err)
	}
	h.body = io.NopCloser(bytes.NewReader(buffer.Bytes()))
	return h
}
func (h *HandlerTestContext) BuildHandler(t *testing.T) (*gin.Context, gin.HandlerFunc, *httptest.ResponseRecorder) {

	request := &http.Request{
		Header: h.headers,
		Method: h.method,
		URL:    h.stubUrl,
		Body:   h.body,
	}

	h.ginContext.Request = request

	for _, p := range h.params {
		h.ginContext.Params = append(h.ginContext.Params, p)
	}

	if h.principal != nil {
		ctx := h.ginContext.Request.Context()
		h.ginContext.Request = h.ginContext.Request.WithContext(iam.DangerouslyWriteUnverifiedPrincipalToContext(ctx, h.principal))
	}

	cfg, err := configureHandler(h.selectedHandler, h.controller, h.logger, h.validate)
	if err != nil {
		t.Fatal("failed to create handler configuration", err)
	}

	return h.ginContext, h.selectedHandler.GetGinHandlerFn(h.logger, h.validate, cfg), h.recorder
}

func ExtractResponseDataAndCode[TYPE any](t *testing.T, r *httptest.ResponseRecorder) (*TYPE, int) {
	var responseBody TYPE
	if err := json.Unmarshal(r.Body.Bytes(), &responseBody); err != nil {
		t.Fatal("failed to unmarshal body", err)
	}
	return &responseBody, r.Code
}
