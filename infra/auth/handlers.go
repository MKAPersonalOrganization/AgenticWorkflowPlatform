package auth

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"school-with-ai/components/auth"
)

// AuthHandlers contains handlers for authentication routes
type AuthHandlers struct {
	kratos *KratosClient
	appURL string
}

// NewAuthHandlers creates new authentication handlers
func NewAuthHandlers(kratos *KratosClient) *AuthHandlers {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		panic("required environment variable APP_URL is not set")
	}
	return &AuthHandlers{
		kratos: kratos,
		appURL: appURL,
	}
}

// LoginPage handles GET /auth/login
func (h *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")
	aal := r.URL.Query().Get("aal")
	refresh := r.URL.Query().Get("refresh") == "true"

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
		slog.Debug("No flow ID found, redirecting to Kratos to initialize login flow")
		initURL := h.kratos.InitFlowURL(FlowTypeLogin, FlowParams{
			ReturnTo: returnTo,
			AAL:      aal,
			Refresh:  refresh,
		})
		http.Redirect(w, r, initURL, http.StatusSeeOther)
		return
	}

	// Fetch the login flow from Kratos
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.GetLoginFlow(r.Context(), flowID, cookie)
	if err != nil {
		slog.Error("Failed to get login flow", "flowID", flowID, "error", err)

		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
			slog.Debug("Flow expired or not found, redirecting to initialize new flow")
			initURL := h.kratos.InitFlowURL(FlowTypeLogin, FlowParams{
				ReturnTo: returnTo,
				AAL:      aal,
				Refresh:  refresh,
			})
			http.Redirect(w, r, initURL, http.StatusSeeOther)
			return
		}

		// Redirect to error page for other errors
		errorURL := h.appURL + "/auth/error?error=login_flow_error"
		http.Redirect(w, r, errorURL, http.StatusSeeOther)
		return
	}

	// Check if verification is required
	if flow.Ui.Messages != nil && ShouldRedirectToVerification(flow.Ui.Messages) {
		slog.Debug("Email verification required, redirecting to verification flow")
		verificationURL := h.kratos.InitFlowURL(FlowTypeVerification, FlowParams{
			ReturnTo: flow.GetReturnTo(),
		})
		http.Redirect(w, r, verificationURL, http.StatusSeeOther)
		return
	}

	// Build recovery and registration URLs
	recoveryQuery := url.Values{}
	if returnTo != "" {
		recoveryQuery.Set("return_to", returnTo)
	} else if flow.GetReturnTo() != "" {
		recoveryQuery.Set("return_to", flow.GetReturnTo())
	}

	registrationQuery := url.Values{}
	if returnTo != "" {
		registrationQuery.Set("return_to", returnTo)
	} else if flow.GetReturnTo() != "" {
		registrationQuery.Set("return_to", flow.GetReturnTo())
	}

	recoveryURL := h.appURL + "/auth/recovery"
	if len(recoveryQuery) > 0 {
		recoveryURL += "?" + recoveryQuery.Encode()
	}

	registrationURL := h.appURL + "/auth/registration"
	if len(registrationQuery) > 0 {
		registrationURL += "?" + registrationQuery.Encode()
	}

	// Don't show recovery link for refresh/2FA flows
	if flow.GetRefresh() || flow.GetRequestedAal() == "aal2" {
		recoveryURL = ""
	}

	// Extract messages from flow
	var messages []auth.UIMessage
	if flow.Ui.Messages != nil {
		for _, msg := range flow.Ui.Messages {
			messages = append(messages, auth.UIMessage{
				ID:      msg.Id,
				Text:    msg.Text,
				Type:    string(msg.Type),
				Context: msg.Context,
			})
		}
	}

	// Render the login page
	props := auth.LoginPageProps{
		FlowAction:      flow.Ui.Action,
		FlowMethod:      flow.Ui.Method,
		Messages:        messages,
		RecoveryURL:     recoveryURL,
		RegistrationURL: registrationURL,
	}

	component := auth.LoginPageWithNodes(flow.Ui.Nodes, props)
	err = component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render login page", "error", err)
	}
}

// RegistrationPage handles GET /auth/registration
func (h *AuthHandlers) RegistrationPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
		slog.Debug("No flow ID found, redirecting to Kratos to initialize registration flow")
		initURL := h.kratos.InitFlowURL(FlowTypeRegistration, FlowParams{
			ReturnTo: returnTo,
		})
		http.Redirect(w, r, initURL, http.StatusSeeOther)
		return
	}

	// Fetch the registration flow from Kratos
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.GetRegistrationFlow(r.Context(), flowID, cookie)
	if err != nil {
		slog.Error("Failed to get registration flow", "flowID", flowID, "error", err)

		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
			slog.Debug("Flow expired or not found, redirecting to initialize new flow")
			initURL := h.kratos.InitFlowURL(FlowTypeRegistration, FlowParams{
				ReturnTo: returnTo,
			})
			http.Redirect(w, r, initURL, http.StatusSeeOther)
			return
		}

		// Redirect to error page for other errors
		errorURL := h.appURL + "/auth/error?error=registration_flow_error"
		http.Redirect(w, r, errorURL, http.StatusSeeOther)
		return
	}

	// Build login URL
	loginQuery := url.Values{}
	if returnTo != "" {
		loginQuery.Set("return_to", returnTo)
	} else if flow.GetReturnTo() != "" {
		loginQuery.Set("return_to", flow.GetReturnTo())
	}

	loginURL := h.appURL + "/auth/login"
	if len(loginQuery) > 0 {
		loginURL += "?" + loginQuery.Encode()
	}

	// Extract messages from flow
	var messages []auth.UIMessage
	if flow.Ui.Messages != nil {
		for _, msg := range flow.Ui.Messages {
			messages = append(messages, auth.UIMessage{
				ID:      msg.Id,
				Text:    msg.Text,
				Type:    string(msg.Type),
				Context: msg.Context,
			})
		}
	}

	// Render the registration page
	props := auth.RegistrationPageProps{
		FlowAction: flow.Ui.Action,
		FlowMethod: flow.Ui.Method,
		Messages:   messages,
		LoginURL:   loginURL,
	}

	component := auth.RegistrationPageWithNodes(flow.Ui.Nodes, props)
	err = component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render registration page", "error", err)
	}
}

// RecoveryPage handles GET /auth/recovery
func (h *AuthHandlers) RecoveryPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Step 16
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// VerificationPage handles GET /auth/verification
func (h *AuthHandlers) VerificationPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
		slog.Debug("No flow ID found, redirecting to Kratos to initialize verification flow")
		initURL := h.kratos.InitFlowURL(FlowTypeVerification, FlowParams{
			ReturnTo: returnTo,
		})
		http.Redirect(w, r, initURL, http.StatusSeeOther)
		return
	}

	// Fetch the verification flow from Kratos
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.GetVerificationFlow(r.Context(), flowID, cookie)
	if err != nil {
		slog.Error("Failed to get verification flow", "flowID", flowID, "error", err)

		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
			slog.Debug("Flow expired or not found, redirecting to initialize new flow")
			initURL := h.kratos.InitFlowURL(FlowTypeVerification, FlowParams{
				ReturnTo: returnTo,
			})
			http.Redirect(w, r, initURL, http.StatusSeeOther)
			return
		}

		// Redirect to error page for other errors
		errorURL := h.appURL + "/auth/error?error=verification_flow_error"
		http.Redirect(w, r, errorURL, http.StatusSeeOther)
		return
	}

	// Build login URL
	loginQuery := url.Values{}
	if returnTo != "" {
		loginQuery.Set("return_to", returnTo)
	} else if flow.GetReturnTo() != "" {
		loginQuery.Set("return_to", flow.GetReturnTo())
	}

	loginURL := h.appURL + "/auth/login"
	if len(loginQuery) > 0 {
		loginURL += "?" + loginQuery.Encode()
	}

	// Extract messages from flow
	var messages []auth.UIMessage
	if flow.Ui.Messages != nil {
		for _, msg := range flow.Ui.Messages {
			messages = append(messages, auth.UIMessage{
				ID:      msg.Id,
				Text:    msg.Text,
				Type:    string(msg.Type),
				Context: msg.Context,
			})
		}
	}

	// Render the verification page
	props := auth.VerificationPageProps{
		FlowAction: flow.Ui.Action,
		FlowMethod: flow.Ui.Method,
		Messages:   messages,
		LoginURL:   loginURL,
	}

	component := auth.VerificationPageWithNodes(flow.Ui.Nodes, props)
	err = component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render verification page", "error", err)
	}
}

// SettingsPage handles GET /auth/settings
func (h *AuthHandlers) SettingsPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Step 17
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// LogoutHandler handles GET /auth/logout
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Step 18
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// ErrorPage handles GET /auth/error
func (h *AuthHandlers) ErrorPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Step 18
	http.Error(w, "Authentication error", http.StatusBadRequest)
}
