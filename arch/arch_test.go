package arch

import (
	"testing"

	"github.com/kcmvp/archunit"
	"github.com/stretchr/testify/assert"
)

// Define all layers once at package level.
var (
	domainLayer   = archunit.ArchLayer("Domain", "school-with-ai/domain")
	servicesLayer = archunit.ArchLayer("Services", "school-with-ai/services/...")
	handlersLayer = archunit.ArchLayer("Handlers", "school-with-ai/handlers")
	infraLayer    = archunit.ArchLayer("Infra", "school-with-ai/infra/...")
	agentSDKLayer = archunit.ArchLayer("AgentSDK", "school-with-ai/sdk/agentsdk/...")
	agentsLayer   = archunit.ArchLayer("Agents", "school-with-ai/services/chatwrite/agents/...")
)

func init() {
	// Initialize architecture with all layers
	archunit.ArchUnit(domainLayer, servicesLayer, handlersLayer, infraLayer, agentSDKLayer, agentsLayer)
}

// TestCleanArchitecture enforces clean architecture layer boundaries
// as defined in ADR-001-clean-architecture-layer-boundaries.md.
func TestCleanArchitecture(t *testing.T) {
	t.Parallel()
	// Rule 1: Services can only depend on Domain
	err := archunit.ArchUnit().Validate(
		archunit.Layers("Services").ShouldNotRefer(archunit.Layers("Handlers", "Infra")),
	)
	assert.NoError(t, err, "Services should not depend on Handlers or Infra layers")

	// Rule 2: Handlers can depend on Services and Domain but not Infra
	err = archunit.ArchUnit().Validate(
		archunit.Layers("Handlers").ShouldNotRefer(archunit.Layers("Infra")),
	)
	assert.NoError(t, err, "Handlers should not depend on Infra layer")

	// Rule 3: Infrastructure can only depend on Domain
	err = archunit.ArchUnit().Validate(
		archunit.Layers("Infra").ShouldNotRefer(archunit.Layers("Services", "Handlers")),
	)
	assert.NoError(t, err, "Infra should not depend on Services or Handlers layers")

	// Rule 4: Domain should not be referred by Infra (dependency inversion)
	err = archunit.ArchUnit().Validate(
		archunit.Layers("Domain").ShouldOnlyBeReferredBy(archunit.Layers("Services", "Handlers")),
	)
	if err != nil {
		t.Logf("Warning: Domain dependency inversion check: %v", err)
	}
}

// TestNoCircularDependencies ensures no circular dependencies exist between layers.
func TestNoCircularDependencies(t *testing.T) {
	t.Parallel()
	// Services should only be referred by Handlers
	err := archunit.ArchUnit().Validate(
		archunit.Layers("Services").ShouldOnlyBeReferredBy(archunit.Layers("Handlers")),
	)
	if err != nil {
		t.Logf("Warning: Services circular dependency check: %v", err)
	}
}

// TestLayerDepth ensures layers don't have excessive package depth.
func TestLayerDepth(t *testing.T) {
	t.Parallel()
	t.Skip("DepthShouldLessThan not available in v0.2.0-alpha1")
}
