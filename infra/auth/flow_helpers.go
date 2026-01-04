package auth

import (
	"fmt"
	"net/url"
	"strings"

	client "github.com/ory/client-go"
)

// FlowType represents the different self-service flow types
type FlowType string

const (
	FlowTypeLogin        FlowType = "login"
	FlowTypeRegistration FlowType = "registration"
	FlowTypeRecovery     FlowType = "recovery"
	FlowTypeVerification FlowType = "verification"
	FlowTypeSettings     FlowType = "settings"
)

// FlowParams contains parameters for initializing or fetching a flow
type FlowParams struct {
	FlowID         string
	ReturnTo       string
	AAL            string
	Refresh        bool
	Organization   string
	Via            string
	LoginChallenge string
	Cookie         string
}

// InitFlowURL generates the URL to initialize a new flow
func (k *KratosClient) InitFlowURL(flowType FlowType, params FlowParams) string {
	query := url.Values{}

	if params.ReturnTo != "" {
		query.Set("return_to", params.ReturnTo)
	}
	if params.AAL != "" {
		query.Set("aal", params.AAL)
	}
	if params.Refresh {
		query.Set("refresh", "true")
	}
	if params.Organization != "" {
		query.Set("organization", params.Organization)
	}
	if params.Via != "" {
		query.Set("via", params.Via)
	}
	if params.LoginChallenge != "" {
		query.Set("login_challenge", params.LoginChallenge)
	}

	baseURL := k.browserURL
	flowPath := fmt.Sprintf("/self-service/%s/browser", flowType)

	if len(query) > 0 {
		return fmt.Sprintf("%s%s?%s", baseURL, flowPath, query.Encode())
	}
	return fmt.Sprintf("%s%s", baseURL, flowPath)
}

// ExtractCSRFToken extracts the CSRF token from flow UI nodes
func ExtractCSRFToken(nodes []client.UiNode) string {
	for _, node := range nodes {
		if node.Group == "default" {
			// Try with UiNodeInputAttributes type
			if node.Attributes.UiNodeInputAttributes != nil {
				attrs := node.Attributes.UiNodeInputAttributes
				if attrs.Name == "csrf_token" {
					if attrs.Value != nil {
						if value, ok := attrs.Value.(string); ok {
							return value
						}
					}
				}
			}
		}
	}
	return ""
}

// UIMessage represents a user-facing message from Kratos
type UIMessage struct {
	ID      int64
	Text    string
	Type    string
	Context map[string]interface{}
}

// ExtractMessages extracts and formats UI messages from Kratos flow
func ExtractMessages(messages []client.UiText) []UIMessage {
	result := make([]UIMessage, 0, len(messages))

	for _, msg := range messages {
		uiMsg := UIMessage{
			ID:      msg.Id,
			Text:    msg.Text,
			Type:    string(msg.Type),
			Context: msg.Context,
		}
		result = append(result, uiMsg)
	}

	return result
}

// GetMessageType returns a CSS-friendly message type (error, success, info, warning)
func GetMessageType(msgType string) string {
	msgType = strings.ToLower(msgType)
	switch msgType {
	case "error":
		return "error"
	case "success":
		return "success"
	case "info":
		return "info"
	default:
		return "info"
	}
}

// IsFlowExpired checks if a flow has expired based on common error codes
func IsFlowExpired(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	// Kratos returns 410 Gone for expired flows
	// or 404 Not Found for non-existent flows
	return strings.Contains(errMsg, "410") ||
		strings.Contains(errMsg, "404") ||
		strings.Contains(errMsg, "flow expired") ||
		strings.Contains(errMsg, "flow not found")
}

// ShouldRedirectToVerification checks if login flow requires email verification
// Message ID 4000010 indicates verification is required
func ShouldRedirectToVerification(messages []client.UiText) bool {
	for _, msg := range messages {
		if msg.Id == 4000010 {
			return true
		}
	}
	return false
}

// IsAAL2Required checks if the error indicates AAL2 (2FA) is required
func IsAAL2Required(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "session_aal2_required") ||
		strings.Contains(errMsg, "aal2")
}

// GetNodeValue extracts the value from a UI node by name
func GetNodeValue(nodes []client.UiNode, name string) string {
	for _, node := range nodes {
		if node.Attributes.UiNodeInputAttributes != nil {
			attrs := node.Attributes.UiNodeInputAttributes
			if attrs.Name == name && attrs.Value != nil {
				if value, ok := attrs.Value.(string); ok {
					return value
				}
			}
		}
	}
	return ""
}

// FilterNodesByGroup filters UI nodes by group (e.g., "default", "password", "profile")
func FilterNodesByGroup(nodes []client.UiNode, groups ...string) []client.UiNode {
	if len(groups) == 0 {
		return nodes
	}

	groupSet := make(map[string]bool)
	for _, g := range groups {
		groupSet[g] = true
	}

	filtered := make([]client.UiNode, 0)
	for _, node := range nodes {
		if groupSet[node.Group] {
			filtered = append(filtered, node)
		}
	}

	return filtered
}
