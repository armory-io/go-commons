package proxy

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const (
	organizationID = "0043dba7-200b-4bea-857b-a1b060089197"
	environmentID  = "6604133f-f21a-4d38-8d76-b5923874cfd9"
)

func TestClientRetry(t *testing.T) {
	counter := 3
	wormhole := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if counter > 0 {
			counter--
			writer.WriteHeader(500)
			return
		}

		if err := json.NewEncoder(writer).Encode(&KubernetesCredentials{
			Host: "success",
		}); err != nil {
			writer.WriteHeader(500)
		}
	}))

	client := New(wormhole.URL, &SessionOverrides{}, func() (string, error) {
		return "", nil
	})

	creds, err := client.GetKubernetesClusterCredentialsFromAgent(context.Background(), &AgentGroup{
		AgentIdentifier: "my-agent",
		OrganizationId:  "org-id",
		EnvironmentId:   "env-id",
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", creds.Host)
}

func TestProxiedClient(t *testing.T) {
	token := os.Getenv("ARMORY_CLOUD_TOKEN")
	if len(token) < 1 {
		t.Skip()
	}

	wormhole := &WormholeService{
		WormholeBaseUrl: "https://localhost:8080",
		TokenSupplier: func() (string, error) {
			return token, nil
		},
		SessionOverrides: &SessionOverrides{Host: "localhost"},
	}

	agentGroup := &AgentGroup{
		AgentIdentifier: "cdf-eks-rna",
		OrganizationId:  organizationID,
		EnvironmentId:   environmentID,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config, err := wormhole.GetProxyEnabledClusterConfig(ctx, agentGroup)
	if err != nil {
		t.Fatalf("Could not create wormhole-enabled proxy: %s", err)
	}

	// create the clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Could not create clientset: %s", err)
	}

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Could not load pods: %s", err)
	}
	t.Logf("There are %d pods in the cluster\n", len(pods.Items))

	if len(pods.Items) < 1 {
		t.Fatalf("Expected there to be at least 1 pod in the result, this probably means something is broken")
	}

	time.Sleep(5 * time.Second)

	pods, err = client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Could not load pods: %s", err)
	}
	t.Logf("There are %d pods in the cluster\n", len(pods.Items))
}

func TestGetProxyEnabledClusterConfigError(t *testing.T) {
	token := os.Getenv("ARMORY_CLOUD_TOKEN")
	if len(token) < 1 {
		t.Skip()
	}

	wormhole := &WormholeService{
		WormholeBaseUrl: "https://localhost:8080",
		TokenSupplier: func() (string, error) {
			return token, nil
		},
		SessionOverrides: &SessionOverrides{Host: "localhost"},
	}

	agentGroup := &AgentGroup{
		AgentIdentifier: "does-not-exist",
		OrganizationId:  organizationID,
		EnvironmentId:   environmentID,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := wormhole.GetProxyEnabledClusterConfig(ctx, agentGroup)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestGetProxyFunctionError(t *testing.T) {
	token := os.Getenv("ARMORY_CLOUD_TOKEN")
	if len(token) < 1 {
		t.Skip()
	}

	wormhole := &WormholeService{
		WormholeBaseUrl: "https://localhost:8080",
		TokenSupplier: func() (string, error) {
			return token, nil
		},
		SessionOverrides: &SessionOverrides{Host: "localhost"},
	}

	agentGroup := &AgentGroup{
		AgentIdentifier: "does-not-exist",
		OrganizationId:  organizationID,
		EnvironmentId:   environmentID,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := wormhole.GetProxyFunction(ctx, agentGroup)
	assert.NotNil(t, err)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestListAgents(t *testing.T) {
	token := os.Getenv("ARMORY_CLOUD_TOKEN")
	if len(token) < 1 {
		t.Skip()
	}

	wormhole := &WormholeService{
		WormholeBaseUrl: "https://localhost:8080",
		TokenSupplier: func() (string, error) {
			return token, nil
		},
		SessionOverrides: &SessionOverrides{Host: "localhost"},
	}

	agents, err := wormhole.ListAgents(context.Background(), organizationID, environmentID)
	if err != nil {
		t.Fatalf("Could not list agents: %s", err)
	}

	assert.Greater(t, len(agents), 1)
}
