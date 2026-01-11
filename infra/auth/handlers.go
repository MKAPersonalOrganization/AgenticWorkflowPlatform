package auth

import (
	"net/http"
	"net/url"
	"os"

	"school-with-ai/components/auth"
	"school-with-ai/foundation/logger"
)

// AuthHandlers contains handlers for authentication routes.
type AuthHandlers struct {
	log    *logger.Logger
	kratos *KratosClient
	appURL string
}

// NewAuthHandlers creates new authentication handlers.
func NewAuthHandlers(log *logger.Logger, kratos *KratosClient) *AuthHandlers {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		panic("required environment variable APP_URL is not set")
	}

	return &AuthHandlers{
		log:    log,
		kratos: kratos,
		appURL: appURL,
	}
}

// LoginPage handles GET /auth/login.
func (h *AuthHandlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")
	aal := r.URL.Query().Get("aal")
	refresh := r.URL.Query().Get("refresh") == "true"

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
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
		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
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
		User:            GetUserFromContext(r.Context()), // nil if not authenticated
	}

	component := auth.LoginPageWithNodes(flow.Ui.Nodes, props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render login page", "error", err, "flowID", flowID)
	}
}

// RegistrationPage handles GET /auth/registration.
func (h *AuthHandlers) RegistrationPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
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
		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
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
		User:       GetUserFromContext(r.Context()), // nil if not authenticated
	}

	component := auth.RegistrationPageWithNodes(flow.Ui.Nodes, props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render registration page", "error", err, "flowID", flowID)
	}
}

// RecoveryPage handles GET /auth/recovery.
func (h *AuthHandlers) RecoveryPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
		initURL := h.kratos.InitFlowURL(FlowTypeRecovery, FlowParams{
			ReturnTo: returnTo,
		})
		http.Redirect(w, r, initURL, http.StatusSeeOther)

		return
	}

	// Fetch the recovery flow from Kratos
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.GetRecoveryFlow(r.Context(), flowID, cookie)
	if err != nil {
		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
			initURL := h.kratos.InitFlowURL(FlowTypeRecovery, FlowParams{
				ReturnTo: returnTo,
			})
			http.Redirect(w, r, initURL, http.StatusSeeOther)

			return
		}

		// Redirect to error page for other errors
		errorURL := h.appURL + "/auth/error?error=recovery_flow_error"
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

	// Render the recovery page
	props := auth.RecoveryPageProps{
		FlowAction: flow.Ui.Action,
		FlowMethod: flow.Ui.Method,
		Messages:   messages,
		LoginURL:   loginURL,
		User:       GetUserFromContext(r.Context()), // nil if not authenticated
	}

	component := auth.RecoveryPageWithNodes(flow.Ui.Nodes, props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render recovery page", "error", err, "flowID", flowID)
	}
}

// VerificationPage handles GET /auth/verification.
func (h *AuthHandlers) VerificationPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
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
		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
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
		User:       GetUserFromContext(r.Context()), // nil if not authenticated
	}

	component := auth.VerificationPageWithNodes(flow.Ui.Nodes, props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render verification page", "error", err, "flowID", flowID)
	}
}

// SettingsPage handles GET /auth/settings.
func (h *AuthHandlers) SettingsPage(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flow")
	returnTo := r.URL.Query().Get("return_to")

	// If no flow ID, redirect to Kratos to initialize flow
	if flowID == "" {
		initURL := h.kratos.InitFlowURL(FlowTypeSettings, FlowParams{
			ReturnTo: returnTo,
		})
		http.Redirect(w, r, initURL, http.StatusSeeOther)

		return
	}

	// Fetch the settings flow from Kratos
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.GetSettingsFlow(r.Context(), flowID, cookie)
	if err != nil {
		// If flow expired or not found, redirect to initialize new flow
		if IsFlowExpired(err) {
			initURL := h.kratos.InitFlowURL(FlowTypeSettings, FlowParams{
				ReturnTo: returnTo,
			})
			http.Redirect(w, r, initURL, http.StatusSeeOther)

			return
		}

		// Redirect to error page for other errors
		errorURL := h.appURL + "/auth/error?error=settings_flow_error"
		http.Redirect(w, r, errorURL, http.StatusSeeOther)

		return
	}

	// Build logout URL
	logoutURL := h.appURL + "/auth/logout"

	// Get authenticated user from context
	user := GetUserFromContext(r.Context())

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

	// Render the settings page
	props := auth.SettingsPageProps{
		FlowAction: flow.Ui.Action,
		FlowMethod: flow.Ui.Method,
		Messages:   messages,
		LogoutURL:  logoutURL,
		User:       user,
	}

	component := auth.SettingsPageWithNodes(flow.Ui.Nodes, props)
	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render settings page", "error", err, "flowID", flowID)
	}
}

// LogoutHandler handles GET /auth/logout.
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Get the logout token from query params (if redirected from Kratos)
	logoutToken := r.URL.Query().Get("token")

	if logoutToken != "" {
		// We have a logout token, perform the actual logout
		// This is the final step where we submit the logout token to Kratos
		// The logout URL is in the Kratos public API
		logoutURL := h.kratos.PublicURL() + "/self-service/logout?token=" + logoutToken
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)

		return
	}

	// No token - create a new logout flow
	cookie := r.Header.Get("Cookie")
	flow, err := h.kratos.CreateBrowserLogoutFlow(r.Context(), cookie)
	if err != nil {
		// On error, just redirect to login
		http.Redirect(w, r, h.appURL+"/auth/login", http.StatusSeeOther)

		return
	}

	// Redirect to the logout URL which will complete the logout
	// Kratos will then redirect back to our app
	http.Redirect(w, r, flow.LogoutUrl, http.StatusSeeOther)
}

// ErrorPage handles GET /auth/error.
func (h *AuthHandlers) ErrorPage(w http.ResponseWriter, r *http.Request) {
	errorID := r.URL.Query().Get("id")
	errorParam := r.URL.Query().Get("error")

	var errorMessage, errorDetails string

	// If we have an error ID, try to fetch the error from Kratos
	if errorID != "" {
		flowError, err := h.kratos.GetFlowError(r.Context(), errorID)
		if err != nil {
			errorMessage = "An unexpected error occurred"
			errorDetails = "We couldn't retrieve the error details. Please try again."
		} else if flowError.Error != nil {
			// Extract message from the error map
			if msg, ok := flowError.Error["message"].(string); ok {
				errorMessage = msg
			}
			if reason, ok := flowError.Error["reason"].(string); ok {
				errorDetails = reason
			}
			// Also try "error_description" for OAuth2 errors
			if errorMessage == "" {
				if desc, ok := flowError.Error["error_description"].(string); ok {
					errorMessage = desc
				}
			}
		}
	} else if errorParam != "" {
		// Use the error parameter to show a generic message
		errorMessage = getErrorMessage(errorParam)
	} else {
		// No error info provided
		errorMessage = "An unexpected error occurred"
		errorDetails = "Please try again or contact support if the problem persists."
	}

	// Build URLs
	loginURL := h.appURL + "/auth/login"
	homeURL := h.appURL + "/"

	// Render the error page
	props := auth.ErrorPageProps{
		ErrorID:      errorID,
		ErrorMessage: errorMessage,
		ErrorDetails: errorDetails,
		LoginURL:     loginURL,
		HomeURL:      homeURL,
		User:         GetUserFromContext(r.Context()), // nil if not authenticated
	}

	component := auth.ErrorPage(props)
	err := component.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// getErrorMessage returns a user-friendly error message for common error types.
func getErrorMessage(errorType string) string {
	switch errorType {
	case "login_flow_error":
		return "Login Error"
	case "registration_flow_error":
		return "Registration Error"
	case "recovery_flow_error":
		return "Password Recovery Error"
	case "verification_flow_error":
		return "Email Verification Error"
	case "settings_flow_error":
		return "Settings Error"
	case "session_error":
		return "Session Error"
	default:
		return "Authentication Error"
	}
}
