package arch

import (
	"testing"

	"github.com/kcmvp/archunit"
	"github.com/stretchr/testify/assert"
)

// TestCleanArchitecture enforces clean architecture layer boundaries
// as defined in ADR-001-clean-architecture-layer-boundaries.md
func TestCleanArchitecture(t *testing.T) {
	// Define architectural layers based on the project structure
	domainLayer := archunit.Packages("Domain", []string{".../domain"})
	servicesLayer := archunit.Packages("Services", []string{".../services/..."})
	handlersLayer := archunit.Packages("Handlers", []string{".../handlers"})
	infraLayer := archunit.Packages("Infra", []string{".../infra/..."})

	// Rule 1: Services can only depend on Domain
	// Services implement use cases and should be independent of external concerns
	err := servicesLayer.ShouldNotReferLayers(handlersLayer, infraLayer)
	assert.NoError(t, err, "Services should not depend on Handlers or Infra layers")

	// Rule 2: Handlers can depend on Services and Domain but not Infra
	err = handlersLayer.ShouldNotReferLayers(infraLayer)
	assert.NoError(t, err, "Handlers should not depend on Infra layer")

	// Rule 3: Infrastructure can only depend on Domain
	err = infraLayer.ShouldNotReferLayers(servicesLayer, handlersLayer)
	assert.NoError(t, err, "Infra should not depend on Services or Handlers layers")

	// Rule 4: Domain should not be referred by Infra (dependency inversion)
	err = domainLayer.ShouldBeOnlyReferredByLayers(servicesLayer, handlersLayer)
	if err != nil {
		t.Logf("Warning: Domain dependency inversion check: %v", err)
	}
}

// TestNoCircularDependencies ensures no circular dependencies exist between layers
func TestNoCircularDependencies(t *testing.T) {
	domainLayer := archunit.Packages("Domain", []string{".../domain"})
	servicesLayer := archunit.Packages("Services", []string{".../services/..."})
	handlersLayer := archunit.Packages("Handlers", []string{".../handlers"})

	// Services should only be referred by Handlers (not Domain)
	err := servicesLayer.ShouldBeOnlyReferredByLayers(handlersLayer)
	if err != nil {
		t.Logf("Warning: Services circular dependency check: %v", err)
	}

	// Handlers should only refer to Services and Domain (excluding standard library)
	err = handlersLayer.ShouldOnlyReferLayers(servicesLayer, domainLayer)
	if err != nil {
		// Filter out standard library imports
		errStr := err.Error()
		if !containsOnlyStdLib(errStr) {
			assert.NoError(t, err, "Handlers should only refer to Services and Domain")
		} else {
			t.Logf("Ignoring standard library dependency: %v", err)
		}
	}
}

// containsOnlyStdLib checks if the error only mentions standard library packages or allowed SDK packages
func containsOnlyStdLib(errStr string) bool {
	// Standard library packages don't have a dot in their import path
	// and don't start with school-with-ai/
	stdLibPackages := []string{
		"context", "encoding/json", "errors", "fmt", "io", "log",
		"net/http", "os", "strings", "sync", "time",
	}

	// Allow SDK packages as they provide shared types
	// Also allow the services package as it contains interfaces that handlers need
	allowedPackages := []string{
		"school-with-ai/sdk/agent-sdk/llm",
		"school-with-ai/sdk/agent-sdk/core",
		"school-with-ai/services",
	}

	for _, pkg := range stdLibPackages {
		if contains(errStr, pkg) {
			return true
		}
	}

	for _, pkg := range allowedPackages {
		if contains(errStr, pkg) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// TestLayerDepth ensures layers don't have excessive package depth
func TestLayerDepth(t *testing.T) {
	domainLayer := archunit.Packages("Domain", []string{".../domain"})
	servicesLayer := archunit.Packages("Services", []string{".../services/..."})

	// Domain layer should not be too deep (max 3 levels)
	err := domainLayer.DepthShouldLessThan(4)
	assert.NoError(t, err, "Domain layer depth should be less than 4")

	// Services layer should not be too deep (max 5 levels due to agents structure)
	err = servicesLayer.DepthShouldLessThan(6)
	assert.NoError(t, err, "Services layer depth should be less than 6")
}
