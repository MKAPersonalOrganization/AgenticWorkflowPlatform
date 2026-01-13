package kratos

import (
	"os"
)

// testConfig creates a test Config from environment variables.
func testConfig() Config {
	return Config{
		KratosPublic:  os.Getenv("KRATOS_PUBLIC_URL"),
		KratosAdmin:   os.Getenv("KRATOS_ADMIN_URL"),
		KratosBrowser: os.Getenv("KRATOS_BROWSER_URL"),
		AppURL:        os.Getenv("APP_URL"),
		AdminAPIToken: os.Getenv("ADMIN_API_TOKEN"),
	}
}
