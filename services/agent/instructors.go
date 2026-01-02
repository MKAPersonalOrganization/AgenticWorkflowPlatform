package agent

// InstructorRegistry manages creation of instructor agents
// Each instructor creates its own tools internally for complete self-containment
type InstructorRegistry struct {
	notifier EventNotifier // Shared UI notifier for all instructors
}

// NewInstructorRegistry creates a new instructor registry
func NewInstructorRegistry(notifier EventNotifier) *InstructorRegistry {
	return &InstructorRegistry{
		notifier: notifier,
	}
}

// GetByName returns the agent configuration for a specific instructor
func (r *InstructorRegistry) GetByName(name string) (AgentConfig, bool) {
	switch name {
	case "student-facing-agent":
		return CreateStudentFacingAgent(r.notifier), true
	case "math-instructor":
		return CreateMathInstructor(r.notifier), true
	case "physics-instructor":
		return CreatePhysicsInstructor(r.notifier), true
	case "chemistry-instructor":
		return CreateChemistryInstructor(r.notifier), true
	case "statistics-instructor":
		return CreateStatisticsInstructor(r.notifier), true
	default:
		return AgentConfig{}, false
	}
}

// GetAllInstructors returns all available instructor configurations
func (r *InstructorRegistry) GetAllInstructors() []AgentConfig {
	return []AgentConfig{
		CreateMathInstructor(r.notifier),
		CreatePhysicsInstructor(r.notifier),
		CreateChemistryInstructor(r.notifier),
		CreateStatisticsInstructor(r.notifier),
	}
}

// GetStudentFacingAgent returns the student-facing agent (entry point for all conversations)
func (r *InstructorRegistry) GetStudentFacingAgent() AgentConfig {
	return CreateStudentFacingAgent(r.notifier)
}

// SetNotifier updates the notifier for all agents created by this registry
// This allows dynamically setting a per-request notifier (e.g., for SSE streams)
func (r *InstructorRegistry) SetNotifier(notifier EventNotifier) {
	r.notifier = notifier
}
