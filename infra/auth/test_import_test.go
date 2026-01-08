package auth

import (
	"testing"

	client "github.com/ory/client-go"
)

func TestImportOryClient(t *testing.T) {
	t.Parallel()
	config := client.NewConfiguration()
	if config == nil {
		t.Fatal("Expected configuration to be non-nil")
	}
}

func TestNewConfigurationWithBasePath(t *testing.T) {
	t.Parallel()
	config := client.NewConfiguration()
	config.Servers = client.ServerConfigurations{
		{
			URL: "http://localhost:4433",
		},
	}

	if len(config.Servers) == 0 {
		t.Fatal("Expected servers to be configured")
	}

	if config.Servers[0].URL != "http://localhost:4433" {
		t.Errorf("Expected server URL to be http://localhost:4433, got %s", config.Servers[0].URL)
	}
}
