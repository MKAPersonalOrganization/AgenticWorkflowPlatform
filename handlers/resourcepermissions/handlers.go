package resourcepermissions

import (
	"encoding/json"
	"fmt"
	"net/http"

	"school-with-ai/shared"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	resourcesmod "school-with-ai/modules/resources"
	"school-with-ai/services/resourcepermissions"

	"github.com/google/uuid"
)

// Handler provides HTTP endpoints for resource permission management.
type Handler struct {
	log                 *logger.Logger
	resourcePermService *resourcepermissions.Service
	resourcesModule     *resourcesmod.Module
	agentProvider       agent.AgentProvider
}

// NewHandler creates a new resource permissions handler.
func NewHandler(
	log *logger.Logger,
	resourcePermService *resourcepermissions.Service,
	resourcesModule *resourcesmod.Module,
	agentProvider agent.AgentProvider,
) *Handler {
	return &Handler{
		log:                 log,
		resourcePermService: resourcePermService,
		resourcesModule:     resourcesModule,
		agentProvider:       agentProvider,
	}
}

// AgentResourceResponse represents a resource with its permission state for an agent.
type AgentResourceResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FileType string `json:"file_type"`
	Source   string `json:"source"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
}

// ListAgentResources returns all resources (public+private) with permission state for an agent.
// GET /v1/agents/{agentId}/resources
func (h *Handler) ListAgentResources(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get public resources
	publicResources, err := h.resourcesModule.ListResources(r.Context(), user.ID, resourcesmod.SourcePublic)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list public resources", "error", err)
		http.Error(w, "Failed to retrieve resources", http.StatusInternalServerError)
		return
	}

	// Get private resources
	privateResources, err := h.resourcesModule.ListResources(r.Context(), user.ID, resourcesmod.SourcePrivate)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list private resources", "error", err)
		http.Error(w, "Failed to retrieve resources", http.StatusInternalServerError)
		return
	}

	// Get permissions for this agent
	permissions, err := h.resourcePermService.ListPermissionsByAgent(r.Context(), agentID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list resource permissions", "agent_id", agentID, "error", err)
		http.Error(w, "Failed to retrieve permissions", http.StatusInternalServerError)
		return
	}

	// Build permission lookup
	permMap := make(map[string]bool)
	for _, perm := range permissions {
		permMap[perm.ResourceID] = perm.Enabled
	}

	// Merge all resources with permission state
	allResources := make([]resourcesmod.Resource, 0, len(publicResources)+len(privateResources))
	allResources = append(allResources, publicResources...)
	allResources = append(allResources, privateResources...)

	responseResources := make([]AgentResourceResponse, 0, len(allResources)+100)
	for _, res := range allResources {
		enabled := permMap[res.ID.String()]
		responseResources = append(responseResources, AgentResourceResponse{
			ID:       res.ID.String(),
			Title:    res.Title,
			FileType: res.FileType,
			Source:   string(res.Source),
			Category: "General",
			Enabled:  enabled,
		})
	}

	// Append 100 mock documents for development/demo purposes
	responseResources = append(responseResources, generateMockResources(permMap)...)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"resources": responseResources}); err != nil {
		h.log.Error(r.Context(), "failed to encode resources list", "error", err)
	}
}

// ToggleAgentResourceRequest represents the request body for toggling a resource permission.
type ToggleAgentResourceRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleAgentResource enables or disables a resource for an agent.
// PUT /v1/agents/{agentId}/resources/{resourceId}
func (h *Handler) ToggleAgentResource(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	resourceID := r.PathValue("resourceId")
	if agentID == "" || resourceID == "" {
		http.Error(w, "agentId and resourceId are required", http.StatusBadRequest)
		return
	}

	var req ToggleAgentResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.resourcePermService.SetPermission(r.Context(), agentID, resourceID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update resource permission",
			"agent_id", agentID, "resource_id", resourceID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update resource permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":     "success",
		"resourceId": resourceID,
		"enabled":    req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle resource response", "error", err)
	}
}

// generateMockResources creates 100 mock documents for development/demo.
func generateMockResources(permMap map[string]bool) []AgentResourceResponse {
	mockDocs := []struct {
		title    string
		fileType string
		category string
	}{
		// Mathematics (15)
		{"Algebra Fundamentals", "application/pdf", "Mathematics"},
		{"Calculus I - Limits and Derivatives", "application/pdf", "Mathematics"},
		{"Linear Algebra Notes", "application/pdf", "Mathematics"},
		{"Probability Theory Overview", "application/pdf", "Mathematics"},
		{"Discrete Mathematics Guide", "application/pdf", "Mathematics"},
		{"Geometry Essentials", "application/pdf", "Mathematics"},
		{"Trigonometry Cheat Sheet", "text/plain", "Mathematics"},
		{"Number Theory Introduction", "application/pdf", "Mathematics"},
		{"Statistics for Beginners", "application/pdf", "Mathematics"},
		{"Differential Equations Handbook", "application/pdf", "Mathematics"},
		{"Complex Analysis Notes", "application/pdf", "Mathematics"},
		{"Real Analysis Primer", "application/pdf", "Mathematics"},
		{"Graph Theory Fundamentals", "application/pdf", "Mathematics"},
		{"Abstract Algebra Overview", "application/pdf", "Mathematics"},
		{"Topology Introduction", "text/markdown", "Mathematics"},
		// Physics (15)
		{"Classical Mechanics Notes", "application/pdf", "Physics"},
		{"Thermodynamics Summary", "application/pdf", "Physics"},
		{"Electromagnetism Formulas", "text/plain", "Physics"},
		{"Quantum Mechanics Intro", "application/pdf", "Physics"},
		{"Optics and Waves", "application/pdf", "Physics"},
		{"Relativity Theory Overview", "application/pdf", "Physics"},
		{"Fluid Dynamics Basics", "application/pdf", "Physics"},
		{"Astrophysics Primer", "application/pdf", "Physics"},
		{"Nuclear Physics Guide", "application/pdf", "Physics"},
		{"Particle Physics Summary", "application/pdf", "Physics"},
		{"Solid State Physics Notes", "application/pdf", "Physics"},
		{"Acoustics Fundamentals", "application/pdf", "Physics"},
		{"Plasma Physics Overview", "application/pdf", "Physics"},
		{"Condensed Matter Theory", "application/pdf", "Physics"},
		{"Experimental Physics Lab Manual", "application/pdf", "Physics"},
		// Chemistry (10)
		{"Organic Chemistry Basics", "application/pdf", "Chemistry"},
		{"Inorganic Chemistry Notes", "application/pdf", "Chemistry"},
		{"Physical Chemistry Review", "application/pdf", "Chemistry"},
		{"Biochemistry Fundamentals", "application/pdf", "Chemistry"},
		{"Chemical Bonding Guide", "text/plain", "Chemistry"},
		{"Periodic Table Reference", "application/pdf", "Chemistry"},
		{"Stoichiometry Worksheet", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "Chemistry"},
		{"Lab Safety Manual", "application/pdf", "Chemistry"},
		{"Electrochemistry Notes", "application/pdf", "Chemistry"},
		{"Polymer Chemistry Intro", "application/pdf", "Chemistry"},
		// Computer Science (15)
		{"Data Analysis with Python", "text/markdown", "Computer Science"},
		{"Machine Learning Basics", "application/pdf", "Computer Science"},
		{"Neural Networks Guide", "application/pdf", "Computer Science"},
		{"Deep Learning Handbook", "application/pdf", "Computer Science"},
		{"Natural Language Processing", "application/pdf", "Computer Science"},
		{"Computer Vision Overview", "application/pdf", "Computer Science"},
		{"Reinforcement Learning", "application/pdf", "Computer Science"},
		{"Algorithm Design Manual", "application/pdf", "Computer Science"},
		{"Data Structures Guide", "application/pdf", "Computer Science"},
		{"Operating Systems Concepts", "application/pdf", "Computer Science"},
		{"Python Programming 101", "text/markdown", "Computer Science"},
		{"Go Language Tutorial", "text/markdown", "Computer Science"},
		{"SQL Database Fundamentals", "application/pdf", "Computer Science"},
		{"REST API Design Guide", "text/markdown", "Computer Science"},
		{"Docker and Containers", "text/markdown", "Computer Science"},
		// Biology (10)
		{"Cell Biology Notes", "application/pdf", "Biology"},
		{"Genetics Fundamentals", "application/pdf", "Biology"},
		{"Ecology and Environment", "application/pdf", "Biology"},
		{"Human Anatomy Atlas", "application/pdf", "Biology"},
		{"Microbiology Basics", "application/pdf", "Biology"},
		{"Evolution Theory Summary", "application/pdf", "Biology"},
		{"Botany Field Guide", "application/pdf", "Biology"},
		{"Marine Biology Intro", "application/pdf", "Biology"},
		{"Neuroscience Overview", "application/pdf", "Biology"},
		{"Immunology Primer", "application/pdf", "Biology"},
		// Economics (10)
		{"Macroeconomics Textbook", "application/pdf", "Economics"},
		{"Microeconomics Notes", "application/pdf", "Economics"},
		{"Financial Accounting Guide", "application/pdf", "Economics"},
		{"Marketing Fundamentals", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "Economics"},
		{"Business Strategy Overview", "application/pdf", "Economics"},
		{"Supply Chain Management", "application/pdf", "Economics"},
		{"Entrepreneurship Handbook", "application/pdf", "Economics"},
		{"Project Management Guide", "application/pdf", "Economics"},
		{"Behavioral Economics Intro", "application/pdf", "Economics"},
		{"International Trade Theory", "application/pdf", "Economics"},
		// Languages (10)
		{"Spanish Vocabulary List", "text/plain", "Languages"},
		{"French Grammar Rules", "application/pdf", "Languages"},
		{"German Language Basics", "text/plain", "Languages"},
		{"Mandarin Characters Guide", "application/pdf", "Languages"},
		{"Japanese Hiragana Chart", "application/pdf", "Languages"},
		{"Arabic Alphabet Reference", "text/plain", "Languages"},
		{"English Grammar Handbook", "application/pdf", "Languages"},
		{"Latin Roots and Prefixes", "text/plain", "Languages"},
		{"Korean Hangul Guide", "application/pdf", "Languages"},
		{"Portuguese Conversation Phrases", "text/plain", "Languages"},
		// History (8)
		{"World History Timeline", "application/pdf", "History"},
		{"Ancient Egypt Study Guide", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "History"},
		{"Renaissance Art History", "application/pdf", "History"},
		{"World War II Overview", "application/pdf", "History"},
		{"Cold War Analysis", "application/pdf", "History"},
		{"Ancient Greece Civilization", "application/pdf", "History"},
		{"Industrial Revolution Summary", "application/pdf", "History"},
		{"Medieval Europe Notes", "application/pdf", "History"},
		// Arts (7)
		{"Music Theory Basics", "application/pdf", "Arts"},
		{"Art Composition Guide", "application/pdf", "Arts"},
		{"Film Studies Notes", "application/pdf", "Arts"},
		{"Photography Techniques", "application/pdf", "Arts"},
		{"Graphic Design Principles", "application/pdf", "Arts"},
		{"Color Theory Reference", "text/plain", "Arts"},
		{"Animation Basics", "application/pdf", "Arts"},
	}

	mocks := make([]AgentResourceResponse, 0, len(mockDocs))
	for i, m := range mockDocs {
		id := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(fmt.Sprintf("mock-resource-%d", i))).String()
		source := "public"
		if i%3 == 0 {
			source = "private"
		}
		mocks = append(mocks, AgentResourceResponse{
			ID:       id,
			Title:    m.title,
			FileType: m.fileType,
			Source:   source,
			Category: m.category,
			Enabled:  permMap[id],
		})
	}
	return mocks
}
