package a2a

import (
	"context"
	"fmt"
	"time"
)

// ConnectorStatus represents the lifecycle state of an A2A agent connector.
type ConnectorStatus string

const (
	ConnectorStatusCreated      ConnectorStatus = "created"
	ConnectorStatusDiscovering  ConnectorStatus = "discovering"
	ConnectorStatusConnected    ConnectorStatus = "connected"
	ConnectorStatusAuthRequired ConnectorStatus = "auth_required"
	ConnectorStatusDisconnected ConnectorStatus = "disconnected"
	ConnectorStatusError        ConnectorStatus = "error"
)

// A2AConnector represents a user-registered remote A2A agent endpoint.
type A2AConnector struct {
	ID       string          `json:"id" db:"id"`
	UserID   string          `json:"user_id" db:"user_id"`
	Title    string          `json:"title" db:"title"`
	AgentURL string          `json:"agent_url" db:"agent_url"`
	Status   ConnectorStatus `json:"status" db:"status"`
	ErrorMsg string          `json:"error_msg,omitempty" db:"error_msg"`

	// Disconnect tracking — distinguishes transient failures from permanent ones.
	DisconnectReason    string `json:"disconnect_reason,omitempty" db:"disconnect_reason"`
	DisconnectPermanent bool   `json:"disconnect_permanent,omitempty" db:"disconnect_permanent"`

	// Agent Card fields (populated after successful discovery)
	AgentName        string `json:"agent_name,omitempty" db:"agent_name"`
	AgentDescription string `json:"agent_description,omitempty" db:"agent_description"`
	AgentVersion     string `json:"agent_version,omitempty" db:"agent_version"`
	AgentProvider    string `json:"agent_provider,omitempty" db:"agent_provider"`
	AgentCardJSON    string `json:"-" db:"agent_card_json"`
	Capabilities     string `json:"capabilities,omitempty" db:"capabilities"`
	IconURL          string `json:"icon_url,omitempty" db:"icon_url"`

	// Auth headers (optional — for bearer token or API key auth)
	AuthType   string `json:"auth_type,omitempty" db:"auth_type"`
	AuthToken  string `json:"-" db:"auth_token"`
	AuthHeader string `json:"-" db:"auth_header"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Transition validates and applies a state change. It enforces:
//   - "created" is the initial state and cannot be transitioned to.
//   - moving to "disconnected" requires a reason and a permanence flag.
//   - moving to any other state clears the disconnect fields.
func (c *A2AConnector) Transition(status ConnectorStatus, reason string, permanent bool) error {
	if status == ConnectorStatusCreated {
		return fmt.Errorf("cannot transition back to created state")
	}
	if status == ConnectorStatusDisconnected {
		if reason == "" {
			return fmt.Errorf("disconnect reason is required when transitioning to disconnected")
		}
	} else {
		// Clear disconnect fields when moving to a non-disconnected state.
		reason = ""
		permanent = false
	}
	c.Status = status
	c.DisconnectReason = reason
	c.DisconnectPermanent = permanent
	return nil
}

// A2ADiscoveredSkill represents a skill discovered from a remote A2A agent's card.
type A2ADiscoveredSkill struct {
	ID          string `json:"id" db:"id"`
	ConnectorID string `json:"connector_id" db:"connector_id"`
	SkillID     string `json:"skill_id" db:"skill_id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	Tags        string `json:"tags" db:"tags"`
	Examples    string `json:"examples" db:"examples"`
	InputModes  string `json:"input_modes" db:"input_modes"`
	OutputModes string `json:"output_modes" db:"output_modes"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ConnectorInfo is a lightweight view of an A2AConnector used by workflow
// sub-agent resolution (adkgo.A2ASubAgentProvider).
type ConnectorInfo struct {
	ID          string
	Name        string
	Description string
	AgentURL    string
}

// A2AConnectorRepository defines persistence for A2A agent connectors.
type A2AConnectorRepository interface {
	// Connector CRUD
	Create(ctx context.Context, connector *A2AConnector) error
	GetByID(ctx context.Context, id string) (*A2AConnector, error)
	GetByIDAndUser(ctx context.Context, id, userID string) (*A2AConnector, error)
	ListByUser(ctx context.Context, userID string) ([]A2AConnector, error)
	CountByUser(ctx context.Context, userID string) (int, error)
	Update(ctx context.Context, connector *A2AConnector) error
	Delete(ctx context.Context, id, userID string) error

	// Skill operations
	UpsertSkills(ctx context.Context, connectorID string, skills []A2ADiscoveredSkill) error
	ListSkills(ctx context.Context, connectorID string) ([]A2ADiscoveredSkill, error)
	ListSkillsByConnectors(ctx context.Context, connectorIDs []string) (map[string][]A2ADiscoveredSkill, error)
	DeleteSkillsByConnector(ctx context.Context, connectorID string) error
	CountSkillsByConnectors(ctx context.Context, connectorIDs []string) (map[string]int, error)
}
