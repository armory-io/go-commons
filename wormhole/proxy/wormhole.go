package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/net/http/httpproxy"
	"io"
	"k8s.io/client-go/rest"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	ErrAgentNotFound                      = errors.New("agent not found")
	ErrCredentialFetchNotSupportedByAgent = errors.New("agent does not support credentials fetching")
)

func New(baseURL string, overrides *SessionOverrides, tokenSupplier tokenSupplier) *WormholeService {
	client := &retryablehttp.Client{
		HTTPClient:   cleanhttp.DefaultClient(),
		Logger:       log.New(os.Stderr, "", log.LstdFlags),
		RetryWaitMin: 0,
		RetryWaitMax: 0,
		RetryMax:     20,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}
	return &WormholeService{
		WormholeBaseUrl:  baseURL,
		TokenSupplier:    tokenSupplier,
		SessionOverrides: overrides,
		client:           client.StandardClient(),
	}
}

type tokenSupplier func() (string, error)

type WormholeService struct {
	WormholeBaseUrl  string
	TokenSupplier    tokenSupplier
	SessionOverrides *SessionOverrides
	client           *http.Client
}

type AgentGroup struct {
	AgentIdentifier string `json:"agentIdentifier"`
	OrganizationId  string `json:"orgId"`
	EnvironmentId   string `json:"envId"`
}

type SessionCredentials struct {
	User                  string                `json:"user"`
	Password              string                `json:"password"`
	Host                  string                `json:"host"`
	Port                  int                   `json:"port"`
	ExpiresAt             time.Time             `json:"expiresAtIso8601"`
	KubernetesCredentials KubernetesCredentials `json:"kubernetesCredentials"`
}

type KubernetesCredentials struct {
	Error                        string `json:"error"`
	StackTrace                   string `json:"stackTrace"`
	RootCaBase64EncodedByteArray string `json:"rootCa"`
	TokenBase64EncodedByteArray  string `json:"tokenFile"`
	Host                         string `json:"host"`
	Port                         int32  `json:"port"`
}

type SessionOverrides struct {
	User string
	Host string
	Port int
}

type Agent struct {
	ConnectedAtIso8601     string `json:"connectedAtIso8601,omitempty"`
	NodeIP                 string `json:"nodeIp,omitempty"`
	OrgID                  string `json:"orgId,omitempty"`
	EnvID                  string `json:"envId,omitempty"`
	K8SClusterRoleSupport  bool   `json:"k8sClusterRoleSupport,omitempty"`
	AgentIdentifier        string `json:"agentIdentifier,omitempty"`
	AgentVersion           string `json:"agentVersion,omitempty"`
	IPAddress              string `json:"ipAddress,omitempty"`
	LastHeartbeatAtIso8601 string `json:"lastHeartbeatAtIso8601,omitempty"`
	StreamID               string `json:"streamId,omitempty"`
}

func (ws *WormholeService) getSessionCredentialsForAgentGroup(ctx context.Context, agentGroup *AgentGroup) (*SessionCredentials, error) {
	agentGroupJson, err := json.Marshal(&agentGroup)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ws.WormholeBaseUrl+"/internal/auth/session", bytes.NewBuffer(agentGroupJson))
	if err != nil {
		return nil, err
	}

	token, err := ws.TokenSupplier()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	res, err := ws.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %q", ErrAgentNotFound, agentGroup.AgentIdentifier)
		} else {
			errorBodyBytes, _ := io.ReadAll(res.Body)
			var errBodyAsString string
			if len(errorBodyBytes) > 0 {
				errBodyAsString = string(errorBodyBytes)
			}
			return nil, fmt.Errorf("failed to create socks session credentials, status code: '%d', body: '%s'", res.StatusCode, errBodyAsString)
		}
	}
	var sessionCredentials *SessionCredentials
	if err := json.NewDecoder(res.Body).Decode(&sessionCredentials); err != nil {
		return nil, err
	}
	return sessionCredentials, nil
}

func (ws *WormholeService) getHttpsProxyUrl(ctx context.Context, agentGroup *AgentGroup) (string, error) {
	sessionCredentials, err := ws.getSessionCredentialsForAgentGroup(ctx, agentGroup)
	if err != nil {
		return "", err
	}

	user := sessionCredentials.User
	if ws.SessionOverrides.User != "" {
		user = ws.SessionOverrides.User
	}
	password := sessionCredentials.Password
	host := sessionCredentials.Host
	if ws.SessionOverrides.Host != "" {
		host = ws.SessionOverrides.Host
	}
	port := sessionCredentials.Port
	if ws.SessionOverrides.Port > 0 {
		port = ws.SessionOverrides.Port
	}

	httpsProxyUrl := fmt.Sprintf("socks5://%s:%s@%s:%d", user, password, host, port)
	return httpsProxyUrl, nil
}

func (ws *WormholeService) getProxyConfig(ctx context.Context, agentGroup *AgentGroup) (*httpproxy.Config, error) {
	httpsProxyUrl, err := ws.getHttpsProxyUrl(ctx, agentGroup)
	if err != nil {
		return nil, err
	}
	return &httpproxy.Config{HTTPSProxy: httpsProxyUrl}, nil
}

func (ws *WormholeService) GetProxyFunction(ctx context.Context, agentGroup *AgentGroup) (func(*http.Request) (*url.URL, error), error) {
	proxyConfig, err := ws.getProxyConfig(ctx, agentGroup)
	if err != nil {
		return nil, err
	}
	return func(request *http.Request) (*url.URL, error) {
		return proxyConfig.ProxyFunc()(request.URL)
	}, err
}

func (ws *WormholeService) GetProxyConfiguredTransport(ctx context.Context, agentGroup *AgentGroup) (*http.Transport, error) {
	proxyFunction, err := ws.GetProxyFunction(ctx, agentGroup)
	if err != nil {
		return nil, err
	}
	return &http.Transport{Proxy: proxyFunction}, nil
}

func (ws *WormholeService) GetKubernetesClusterCredentialsFromAgent(ctx context.Context, agentGroup *AgentGroup) (*KubernetesCredentials, error) {
	agentGroupJson, err := json.Marshal(&agentGroup)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ws.WormholeBaseUrl+"/internal/auth/kubernetes-cluster-credentials-for-agent", bytes.NewBuffer(agentGroupJson))
	if err != nil {
		return nil, err
	}

	token, err := ws.TokenSupplier()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	res, err := ws.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		switch res.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("%w: %q", ErrAgentNotFound, agentGroup.AgentIdentifier)
		case http.StatusUnprocessableEntity:
			return nil, fmt.Errorf("%w: %q", ErrCredentialFetchNotSupportedByAgent, agentGroup.AgentIdentifier)
		default:
			errorBodyBytes, _ := io.ReadAll(res.Body)
			var errBodyAsString string
			if len(errorBodyBytes) > 0 {
				errBodyAsString = string(errorBodyBytes)
			}
			return nil, fmt.Errorf("failed to create socks session credentials, status code: '%d', body: '%s'", res.StatusCode, errBodyAsString)
		}
	}

	var credentials *KubernetesCredentials
	if err := json.NewDecoder(res.Body).Decode(&credentials); err != nil {
		return nil, err
	}

	if len(credentials.Error) > 0 {
		trace := "no stack trace present"
		if len(credentials.StackTrace) > 0 {
			trace = credentials.StackTrace
		}
		return nil, fmt.Errorf("failed to fetch Kubernetes credentials, proxy returned wrapped error. err: %s, stacktrace: %s", credentials.Error, trace)
	}

	return credentials, nil
}

func (ws *WormholeService) GetProxyEnabledClusterConfig(ctx context.Context, agentGroup *AgentGroup) (*rest.Config, error) {
	credentials, err := ws.GetKubernetesClusterCredentialsFromAgent(ctx, agentGroup)
	if err != nil {
		return nil, err
	}

	proxyFunction, err := ws.GetProxyFunction(ctx, agentGroup)
	if err != nil {
		return nil, err
	}

	caData, err := base64.StdEncoding.DecodeString(credentials.RootCaBase64EncodedByteArray)
	if err != nil {
		return nil, err
	}

	token, err := base64.StdEncoding.DecodeString(credentials.TokenBase64EncodedByteArray)
	if err != nil {
		return nil, err
	}

	tString := string(token)

	config := &rest.Config{
		Host:            "https://" + net.JoinHostPort(credentials.Host, fmt.Sprintf("%d", credentials.Port)),
		TLSClientConfig: rest.TLSClientConfig{CAData: caData},
		BearerToken:     tString,
		Proxy:           proxyFunction,
	}

	return config, nil
}

func (ws *WormholeService) ListAgents(ctx context.Context, orgID, envID string) ([]*Agent, error) {
	if strings.TrimSpace(orgID) == "" || strings.TrimSpace(envID) == "" {
		return nil, fmt.Errorf("must provide orgID and envID")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ws.WormholeBaseUrl+fmt.Sprintf("/internal/agent-metadata?orgId=%s&envId=%s", orgID, envID), nil)
	if err != nil {
		return nil, err
	}

	token, err := ws.TokenSupplier()
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Accept", "application/json")

	res, err := ws.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		errorBodyBytes, _ := io.ReadAll(res.Body)
		var errBodyAsString string
		if len(errorBodyBytes) > 0 {
			errBodyAsString = string(errorBodyBytes)
		}
		return nil, fmt.Errorf("could not list agents, status code: %d, body: %s", res.StatusCode, errBodyAsString)
	}

	var agents []*Agent
	if err := json.NewDecoder(res.Body).Decode(&agents); err != nil {
		return nil, err
	}
	return agents, nil
}
