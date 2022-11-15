package integration_utils

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"sync"
	"testing"
)

var (
	MasterConnURL string
	Mutex         sync.Mutex
)

type (
	blackholeLogger struct{}
)

func (bl blackholeLogger) Print(v ...interface{}) {}

func CreateIntegrationDatabase(t *testing.T) (endpoint string) {
	Mutex.Lock()
	defer Mutex.Unlock()

	if MasterConnURL != "" {
		return MasterConnURL
	}

	t.Helper()
	_ = mysql.SetLogger(blackholeLogger{})

	ctx := context.Background()

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mysql:5.7",
			ExposedPorts: []string{"3306/tcp"},
			WaitingFor: wait.ForSQL("3306/tcp", "mysql", func(_ string, p nat.Port) string {
				return "test-user:12345@tcp(localhost:" + p.Port() + ")/master"
			}),
			Env: map[string]string{
				"MYSQL_ROOT_PASSWORD": "root",
				"MYSQL_USER":          "test-user",
				"MYSQL_PASSWORD":      "12345",
				"MYSQL_DATABASE":      "master",
			},
		},
	})

	if err != nil {
		t.Fatal(err.Error())
	}

	endpoint, err = c.PortEndpoint(ctx, "3306/tcp", "")
	if err != nil {
		t.Fatal(err.Error())
	}

	MasterConnURL = fmt.Sprintf("root:root@tcp(%s)/master?parseTime=true", endpoint)

	return MasterConnURL
}
