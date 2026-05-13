package documenthandler

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/knowledge/tenant"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// Handler handles document-related HTTP endpoints.
type Handler struct {
	log              *logger.Logger
	documentProvider chatcontracts.DocumentProvider
	tenantID         string
}

// New creates a new document handler.
func New(
	log *logger.Logger,
	documentProvider chatcontracts.DocumentProvider,
	tenantID string,
) *Handler {
	return &Handler{
		log:              log,
		documentProvider: documentProvider,
		tenantID:         tenantID,
	}
}

// ListDocuments handles GET /v1/documents - retrieves documents from Edgequake using passive context resolution.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Resolve Multitenancy Context
	var tCtx *tenant.TenantContext
	if h.tenantID == "" {
		h.log.Error(r.Context(), "Tenant ID not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Static Configuration
	tID, err := uuid.Parse(h.tenantID)
	if err != nil {
		h.log.Error(r.Context(), "Invalid configured tenant ID", "error", err)
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
		return
	}
	tCtx = &tenant.TenantContext{
		TenantID: &tID,
		UserID:   user.ID,
		Role:     string(tenant.RoleAdmin),
	}

	// 3. Delegate to Document Provider (Passive Enforcement)
	// Go does NOT check if the user is allowed to list docs. It just passes the headers.
	resp, err := h.documentProvider.ListDocuments(r.Context(), tCtx, 1, 20, "")
	if err != nil {
		if errors.Is(err, chatcontracts.ErrKnowledgeUnauthorized) {
			http.Error(w, "Unauthorized by Document Provider", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, chatcontracts.ErrKnowledgeForbidden) {
			http.Error(w, "Access forbidden by Document Provider", http.StatusForbidden)
			return
		}
		h.log.Error(r.Context(), "Document Provider API failure", "error", err)
		http.Error(w, "Failed to retrieve documents", http.StatusInternalServerError)
		return
	}

	// 4. Return results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error(r.Context(), "Failed to encode documents response", "error", err)
	}
}

// UploadDocument handles POST /v1/documents - uploads a document to Edgequake using passive context resolution.
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Resolve Multitenancy Context
	var tCtx *tenant.TenantContext
	if h.tenantID == "" {
		h.log.Error(r.Context(), "Tenant ID not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Static Configuration
	tID, err := uuid.Parse(h.tenantID)
	if err != nil {
		h.log.Error(r.Context(), "Invalid configured tenant ID", "error", err)
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
		return
	}
	tCtx = &tenant.TenantContext{
		TenantID: &tID,
		UserID:   user.ID,
		Role:     string(tenant.RoleAdmin),
	}

	// 3. Parse Multipart Form. MaxBytesReader caps the on-wire body before
	// ParseMultipartForm decides how much to keep in memory vs spill.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// 4. Delegate to Document Provider (Passive Enforcement)
	resp, err := h.documentProvider.UploadDocument(r.Context(), tCtx, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		if errors.Is(err, chatcontracts.ErrKnowledgeUnauthorized) {
			http.Error(w, "Unauthorized by Document Provider", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, chatcontracts.ErrKnowledgeForbidden) {
			http.Error(w, "Access forbidden by Document Provider", http.StatusForbidden)
			return
		}
		h.log.Error(r.Context(), "Document Provider API failure during upload", "error", err)
		http.Error(w, "Failed to upload document", http.StatusInternalServerError)
		return
	}

	// 5. Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error(r.Context(), "Failed to encode upload response", "error", err)
	}
}
