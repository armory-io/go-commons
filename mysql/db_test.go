package mysql

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/yaml"
	"testing"
	"time"
)

type Temp struct {
	Dur MDuration `yaml:"dur"`
}

func TestDuration(t *testing.T) {
	cases := []struct {
		name        string
		ser         string
		expectedVal time.Duration
	}{
		{
			"default",
			"",
			0 * time.Minute,
		},
		{
			"10 minutes",
			"dur: 10m",
			10 * time.Minute,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t2 *testing.T) {
			tmp := &Temp{}

			if err := yaml.Unmarshal([]byte(c.ser), tmp); err != nil {
				t2.Fatal("unable to parse input string")
			}
			assert.Equal(t2, c.expectedVal, tmp.Dur.Duration)
		})
	}
}

func TestDuration_Err(t *testing.T) {
	tmp := &Temp{}
	err := yaml.Unmarshal([]byte("dur: not_a_duration"), tmp)
	assert.NotNil(t, err)
}

func TestDatabase_ConnectionUrl(t *testing.T) {
	set := Settings{
		Connection:      "net(localhost:3006)/test",
		User:            "root",
		Password:        "mypassword",
		MigrateUser:     "migrateuser",
		MigratePassword: "migratepwd",
	}
	s, err := set.ConnectionUrl(false)
	assert.Nil(t, err)
	assert.Equal(t, "root:mypassword@net(localhost:3006)/test?parseTime=true", s)

	s, err = set.ConnectionUrl(true)
	assert.Nil(t, err)
	assert.Equal(t, "mysql://migrateuser:migratepwd@net(localhost:3006)/test", s)
}

func TestDatabase_ConnectionUrl2(t *testing.T) {
	set := Settings{
		Connection: "that_is_not_a_connection_string",
	}
	_, err := set.ConnectionUrl(false)
	assert.NotNil(t, err)
}
