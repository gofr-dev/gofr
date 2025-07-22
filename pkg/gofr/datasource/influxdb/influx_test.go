package influxdb

import (
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"testing"
)

func TestAddInfluxDB(t *testing.T) {
	config := New(Config{
		Url:      "http://localhost:8086",
		Username: "admin",
		Password: "admin1234",
		Token:    "F-QFQpmCL9UkR3qyoXnLkzWj03s6m4eCvYgDl1ePfHBf9ph7yxaSgQ6WN0i9giNgRTfONwVMK1f977r_g71oNQ==",
	})

	app := gofr.New()
	app.AddInfluxDB(config)

	//config := Config{
	//	Url:      "http://localhost:8086",
	//	Username: "admin",
	//	Password: "admin1234",
	//	Token:    "F-QFQpmCL9UkR3qyoXnLkzWj03s6m4eCvYgDl1ePfHBf9ph7yxaSgQ6WN0i9giNgRTfONwVMK1f977r_g71oNQ==",
	//}

}

func TestNew(t *testing.T) {
	config := Config{
		Url:      "http://localhost:8086",
		Username: "admin",
		Password: "admin1234",
		Token:    "F-QFQpmCL9UkR3qyoXnLkzWj03s6m4eCvYgDl1ePfHBf9ph7yxaSgQ6WN0i9giNgRTfONwVMK1f977r_g71oNQ==",
	}
	client := New(config)
	require.Equal(t, client.config.Url, config.Url)
	require.Equal(t, client.config.Username, config.Username)
	require.Equal(t, client.config.Password, config.Password)
	require.Equal(t, client.config.Token, config.Token)
}
